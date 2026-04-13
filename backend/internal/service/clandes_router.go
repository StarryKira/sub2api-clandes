package service

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"

	proto "github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// clandesRouterImpl implements proto.Router_Server — the Cap'n Proto callback
// interface that clandes calls into for routing decisions and usage reporting.
type clandesRouterImpl struct {
	reqCache            *clandesRequestCache
	gatewayService      *GatewayService
	billingCacheService *BillingCacheService
	apiKeyService       *APIKeyService
}

func newClandesRouterImpl(
	reqCache *clandesRequestCache,
	gatewaySvc *GatewayService,
	billingSvc *BillingCacheService,
	apiKeySvc *APIKeyService,
) *clandesRouterImpl {
	return &clandesRouterImpl{
		reqCache:            reqCache,
		gatewayService:      gatewaySvc,
		billingCacheService: billingSvc,
		apiKeyService:       apiKeySvc,
	}
}

// RouteRequest is called by clandes before forwarding a request upstream.
// It validates the API key, checks billing eligibility, selects an account,
// and caches context for the subsequent reportUsage call.
//
// Returns empty accountId to reject the request (clandes returns 503 NoRoute).
func (r *clandesRouterImpl) RouteRequest(ctx context.Context, call proto.Router_routeRequest) error {
	args := call.Args()

	requestID, _ := args.RequestId()
	apiKeyStr, _ := args.ApiKey()
	model, _ := args.Model()

	log := logger.L().With(
		zap.String("component", "clandes.router"),
		zap.String("request_id", requestID),
		zap.String("model", model),
	)

	rejectWithEmpty := func() error {
		res, err := call.AllocResults()
		if err != nil {
			return err
		}
		return res.SetAccountId("")
	}

	// 1. Look up API key (uses auth cache)
	apiKey, err := r.apiKeyService.GetByKey(ctx, apiKeyStr)
	if err != nil {
		log.Warn("routeRequest: api key not found", zap.Error(err))
		return rejectWithEmpty()
	}
	if apiKey.User == nil {
		log.Warn("routeRequest: api key has no user")
		return rejectWithEmpty()
	}

	// 2. Check billing eligibility
	var group *Group
	if apiKey.Group != nil {
		group = apiKey.Group
	}
	if err := r.billingCacheService.CheckBillingEligibility(ctx, apiKey.User, apiKey, group, nil); err != nil {
		log.Info("routeRequest: billing eligibility failed", zap.Error(err))
		return rejectWithEmpty()
	}

	// 3. Select account (no session affinity in MVP)
	selection, err := r.gatewayService.SelectAccountWithLoadAwareness(
		ctx,
		apiKey.GroupID,
		"", // no session hash in MVP
		model,
		nil,
		"",
		0,
	)
	if err != nil {
		log.Warn("routeRequest: no available account", zap.Error(err))
		return rejectWithEmpty()
	}
	account := selection.Account
	// Release the slot immediately — clandes manages its own concurrency.
	if selection.Acquired && selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}

	// 4. Cache context for reportUsage
	r.reqCache.set(requestID, &clandesRequestContext{
		APIKey:    apiKey,
		User:      apiKey.User,
		Account:   account,
		GroupID:   apiKey.GroupID,
		StartTime: time.Now(),
	})

	log.Info("routeRequest: routed", zap.Int64("account_id", account.ID))

	res, err := call.AllocResults()
	if err != nil {
		return err
	}
	return res.SetAccountId(strconv.FormatInt(account.ID, 10))
}

// ReportUsage is called by clandes after a request completes (fire-and-forget).
// It maps usage data to sub2api's billing system.
func (r *clandesRouterImpl) ReportUsage(ctx context.Context, call proto.Router_reportUsage) error {
	args := call.Args()

	requestID, _ := args.RequestId()
	report, err := args.Report()
	if err != nil {
		return nil // best-effort
	}

	log := logger.L().With(
		zap.String("component", "clandes.router"),
		zap.String("request_id", requestID),
	)

	// Retrieve cached context from routeRequest
	rctx, ok := r.reqCache.getAndDelete(requestID)
	if !ok {
		log.Warn("reportUsage: no cached context (expired or unknown request_id)")
		return nil
	}

	model, _ := report.Model()
	durationMs := report.DurationMs()
	inputTokens := report.InputTokens()
	outputTokens := report.OutputTokens()
	statusCode := report.StatusCode()

	result := &ForwardResult{
		Model:    model,
		Duration: time.Duration(durationMs) * time.Millisecond,
		Usage: ClaudeUsage{
			InputTokens:  int(inputTokens),
			OutputTokens: int(outputTokens),
			// Cache token fields not in current schema (MVP limitation)
		},
	}

	log.Info("reportUsage: recording",
		zap.String("model", model),
		zap.Int("input_tokens", int(inputTokens)),
		zap.Int("output_tokens", int(outputTokens)),
		zap.Int("status_code", int(statusCode)),
	)

	// Record billing asynchronously to avoid blocking the capnp callback goroutine
	go func() {
		bctx := context.Background()
		if err := r.gatewayService.RecordUsage(bctx, &RecordUsageInput{
			Result:           result,
			APIKey:           rctx.APIKey,
			User:             rctx.User,
			Account:          rctx.Account,
			Subscription:     rctx.Subscription,
			InboundEndpoint:  "/v1/messages",
			UpstreamEndpoint: "/v1/messages",
			APIKeyService:    r.apiKeyService,
		}); err != nil {
			log.Error("reportUsage: RecordUsage failed", zap.Error(err))
		}
	}()

	return nil
}

// ReportChunk is called for each SSE chunk during streaming (fire-and-forget).
// MVP: no-op.
func (r *clandesRouterImpl) ReportChunk(_ context.Context, _ proto.Router_reportChunk) error {
	return nil
}

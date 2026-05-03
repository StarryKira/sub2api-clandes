package service

import (
	"context"
	"fmt"
	"strconv"

	capnp "capnproto.org/go/capnp/v3"
	"go.uber.org/zap"

	proto "github.com/Wei-Shaw/sub2api/internal/pkg/clandes/proto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// SyncAccountsToClandes registers all active accounts marked for clandes routing
// (Anthropic + OpenAI/Codex) into clandes via the AccountService capnp RPC.
func SyncAccountsToClandes(ctx context.Context, client *ClandesClient, accountService *AccountService) error {
	return SyncAccountsByRepo(ctx, client, accountService.accountRepo)
}

// IsClandesAccount returns true if the account is marked for clandes-only routing.
// Clandes accounts have {"clandes": true} in their Extra JSONB field.
// They are managed by clandes for TLS fingerprinting and OAuth refresh;
// sub2api only handles routing decisions and billing.
func IsClandesAccount(acc *Account) bool {
	if acc.Extra == nil {
		return false
	}
	v, ok := acc.Extra["clandes"].(bool)
	return ok && v
}

// clandesSyncedPlatforms lists the platforms whose clandes-flagged accounts should
// be pushed to clandes. Both Anthropic (Claude) and OpenAI (Codex) are synced;
// clandes maps the platform+type combination to its own AccountType internally.
var clandesSyncedPlatforms = []string{PlatformAnthropic, PlatformOpenAI}

// SyncAccountsByRepo is like SyncAccountsToClandes but accepts AccountRepository directly.
// Only syncs accounts marked with {"clandes": true} in Extra.
func SyncAccountsByRepo(ctx context.Context, client *ClandesClient, repo AccountRepository) error {
	svc, err := client.AccountService()
	if err != nil {
		return fmt.Errorf("clandes account sync: get AccountService: %w", err)
	}
	defer svc.Release()

	log := logger.L().With(zap.String("component", "clandes.account_sync"))
	synced, total := 0, 0

	for _, platform := range clandesSyncedPlatforms {
		accounts, err := repo.ListByPlatform(ctx, platform)
		if err != nil {
			return fmt.Errorf("clandes account sync: list %s accounts: %w", platform, err)
		}
		total += len(accounts)
		for i := range accounts {
			acc := &accounts[i]
			if acc.Status != "active" || !IsClandesAccount(acc) {
				continue
			}
			if err := registerAccountToClandes(ctx, svc, acc); err != nil {
				log.Warn("clandes account sync: failed to register account",
					zap.Int64("account_id", acc.ID),
					zap.String("platform", acc.Platform),
					zap.String("account_type", acc.Type),
					zap.Error(err),
				)
				continue
			}
			synced++
		}
	}

	log.Info("clandes account sync: done", zap.Int("synced", synced), zap.Int("total", total))
	return nil
}

// registerAccountToClandes registers a single account to clandes via AccountService.RegisterAccount.
func registerAccountToClandes(ctx context.Context, svc proto.AccountService, acc *Account) error {
	accountType := mapAccountTypeToCapnp(acc)
	if accountType == proto.AccountType_unknown {
		return nil // not a supported type, skip
	}

	fut, rel := svc.RegisterAccount(ctx, func(p proto.AccountService_registerAccount_Params) error {
		if err := p.SetAccountId(strconv.FormatInt(acc.ID, 10)); err != nil {
			return err
		}
		p.SetAccountType(accountType)
		if err := p.SetVersion(""); err != nil { // empty = clandes picks default
			return err
		}
		if acc.Proxy != nil {
			if err := p.SetProxyUrl(BuildProxyURL(acc.Proxy)); err != nil {
				return err
			}
		}

		// NewCredentials allocates the creds struct in the right segment
		creds, err := p.NewCredentials()
		if err != nil {
			return err
		}
		return setAccountCredentials(creds, acc)
	})
	defer rel()

	res, err := fut.Struct()
	if err != nil {
		return fmt.Errorf("RegisterAccount RPC: %w", err)
	}
	if !res.Success() {
		msg, _ := res.Message_()
		return fmt.Errorf("RegisterAccount: %s", msg)
	}
	return nil
}

// RegisterSingleAccountToClandes registers one account to clandes.
// Used when creating a new clandes account from the admin UI.
func RegisterSingleAccountToClandes(ctx context.Context, client *ClandesClient, acc *Account) error {
	svc, err := client.AccountService()
	if err != nil {
		return fmt.Errorf("clandes: get AccountService: %w", err)
	}
	defer svc.Release()
	return registerAccountToClandes(ctx, svc, acc)
}

// RemoveAccountFromClandes removes an account from clandes when it is deleted in sub2api.
func RemoveAccountFromClandes(ctx context.Context, client *ClandesClient, accountID int64) error {
	svc, err := client.AccountService()
	if err != nil {
		return fmt.Errorf("clandes remove account: get AccountService: %w", err)
	}
	defer svc.Release()

	fut, rel := svc.RemoveAccount(ctx, func(p proto.AccountService_removeAccount_Params) error {
		return p.SetAccountId(strconv.FormatInt(accountID, 10))
	})
	defer rel()

	res, err := fut.Struct()
	if err != nil {
		return fmt.Errorf("RemoveAccount RPC: %w", err)
	}
	if !res.Success() {
		msg, _ := res.Message_()
		return fmt.Errorf("RemoveAccount: %s", msg)
	}
	return nil
}

// --- helpers ---

func mapAccountTypeToCapnp(acc *Account) proto.AccountType {
	switch acc.Platform {
	case PlatformAnthropic:
		switch acc.Type {
		case AccountTypeOAuth, AccountTypeSetupToken:
			return proto.AccountType_claudeSubscription
		case AccountTypeAPIKey:
			return proto.AccountType_claudePayAsYouGo
		}
	case PlatformOpenAI:
		switch acc.Type {
		case AccountTypeOAuth:
			return proto.AccountType_codexSubscription
		case AccountTypeAPIKey:
			return proto.AccountType_codexApiKey
		}
	}
	return proto.AccountType_unknown
}

func setAccountCredentials(creds proto.AccountCredentials, acc *Account) error {
	seg := capnp.Struct(creds).Segment()
	switch acc.Platform {
	case PlatformAnthropic:
		return setClaudeCredentials(creds, seg, acc)
	case PlatformOpenAI:
		return setCodexCredentials(creds, seg, acc)
	}
	return nil
}

func setClaudeCredentials(creds proto.AccountCredentials, seg *capnp.Segment, acc *Account) error {
	switch acc.Type {
	case AccountTypeOAuth, AccountTypeSetupToken:
		accessToken := acc.GetCredential("access_token")
		refreshToken := acc.GetCredential("refresh_token")
		tokenCreds, err := proto.NewClaudeTokenCredentials(seg)
		if err != nil {
			return err
		}
		if err := tokenCreds.SetAccessToken(accessToken); err != nil {
			return err
		}
		if err := tokenCreds.SetRefreshToken(refreshToken); err != nil {
			return err
		}
		return creds.SetClaudeSubCreds(tokenCreds)

	case AccountTypeAPIKey:
		apiKey := acc.GetCredential("api_key")
		baseURL := acc.GetCredential("base_url")
		keyCreds, err := proto.NewClaudeApiKeyCredentials(seg)
		if err != nil {
			return err
		}
		if err := keyCreds.SetApiKey(apiKey); err != nil {
			return err
		}
		if err := keyCreds.SetBaseUrl(baseURL); err != nil {
			return err
		}
		return creds.SetClaudeApiKeyCreds(keyCreds)
	}
	return nil
}

func setCodexCredentials(creds proto.AccountCredentials, seg *capnp.Segment, acc *Account) error {
	switch acc.Type {
	case AccountTypeOAuth:
		accessToken := acc.GetCredential("access_token")
		refreshToken := acc.GetCredential("refresh_token")
		idToken := acc.GetCredential("id_token")
		tokenCreds, err := proto.NewCodexOAuthCredentials(seg)
		if err != nil {
			return err
		}
		if err := tokenCreds.SetAccessToken(accessToken); err != nil {
			return err
		}
		if err := tokenCreds.SetRefreshToken(refreshToken); err != nil {
			return err
		}
		if err := tokenCreds.SetIdToken(idToken); err != nil {
			return err
		}
		return creds.SetCodexOAuthCreds(tokenCreds)

	case AccountTypeAPIKey:
		apiKey := acc.GetCredential("api_key")
		baseURL := acc.GetCredential("base_url")
		keyCreds, err := proto.NewCodexApiKeyCredentials(seg)
		if err != nil {
			return err
		}
		if err := keyCreds.SetApiKey(apiKey); err != nil {
			return err
		}
		if err := keyCreds.SetBaseUrl(baseURL); err != nil {
			return err
		}
		return creds.SetCodexApiKeyCreds(keyCreds)
	}
	return nil
}

// BuildProxyURL builds a raw proxy URL for clandes (no URL-encoding of credentials).
func BuildProxyURL(proxy *Proxy) string {
	if proxy == nil {
		return ""
	}
	scheme := proxy.Protocol
	if scheme == "" {
		scheme = "http"
	}
	if proxy.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d", scheme, proxy.Username, proxy.Password, proxy.Host, proxy.Port)
	}
	return fmt.Sprintf("%s://%s:%d", scheme, proxy.Host, proxy.Port)
}

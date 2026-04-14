package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ClandesHandler exposes clandes integration status and control to the admin UI.
type ClandesHandler struct {
	client      *service.ClandesClient   // may be nil when clandes.enabled=false
	accountRepo service.AccountRepository
}

// NewClandesHandler creates a ClandesHandler.
// client may be nil if clandes integration is disabled.
func NewClandesHandler(client *service.ClandesClient, accountRepo service.AccountRepository) *ClandesHandler {
	return &ClandesHandler{
		client:      client,
		accountRepo: accountRepo,
	}
}

// GetStatus returns the current clandes integration status.
// GET /api/v1/admin/clandes/status
func (h *ClandesHandler) GetStatus(c *gin.Context) {
	if h.client == nil {
		response.Success(c, service.ClandesStatus{
			Enabled:   false,
			Connected: false,
			Addr:      "",
		})
		return
	}
	response.Success(c, h.client.Status())
}

// SyncAccounts triggers a manual re-sync of clandes-flagged accounts.
// POST /api/v1/admin/clandes/sync
func (h *ClandesHandler) SyncAccounts(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	if err := service.SyncAccountsByRepo(c.Request.Context(), h.client, h.accountRepo); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "accounts synced successfully"})
}

// ListAccounts returns all accounts marked with extra.clandes=true.
// GET /api/v1/admin/clandes/accounts
func (h *ClandesHandler) ListAccounts(c *gin.Context) {
	accounts, err := h.accountRepo.ListByPlatform(c.Request.Context(), service.PlatformAnthropic)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	var result []service.Account
	for i := range accounts {
		if service.IsClandesAccount(&accounts[i]) {
			result = append(result, accounts[i])
		}
	}
	response.Success(c, result)
}

// createClandesAccountRequest is the request body for creating a clandes account.
type createClandesAccountRequest struct {
	Name        string         `json:"name" binding:"required"`
	Type        string         `json:"type" binding:"required,oneof=oauth setup-token apikey"`
	Credentials map[string]any `json:"credentials" binding:"required"`
	ProxyID     *int64         `json:"proxy_id"`
	GroupIDs    []int64        `json:"group_ids"`
}

// CreateAccount creates an account marked for clandes-only routing
// and registers it to clandes via capnp.
// POST /api/v1/admin/clandes/accounts
func (h *ClandesHandler) CreateAccount(c *gin.Context) {
	var req createClandesAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	account := &service.Account{
		Name:        req.Name,
		Platform:    service.PlatformAnthropic,
		Type:        req.Type,
		Credentials: req.Credentials,
		Extra:       map[string]any{"clandes": true},
		ProxyID:     req.ProxyID,
		Concurrency: 1,
		Status:      "active",
	}

	if err := h.accountRepo.Create(c.Request.Context(), account); err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("create account: %v", err))
		return
	}

	// Bind to groups if specified
	if len(req.GroupIDs) > 0 {
		if err := h.accountRepo.BindGroups(c.Request.Context(), account.ID, req.GroupIDs); err != nil {
			response.Error(c, http.StatusInternalServerError, fmt.Sprintf("bind groups: %v", err))
			return
		}
	}

	// Register to clandes if connected
	if h.client != nil {
		if err := service.RegisterSingleAccountToClandes(c.Request.Context(), h.client, account); err != nil {
			// Non-fatal: account is created locally, sync can retry later
			c.Writer.Header().Set("X-Clandes-Sync-Warning", err.Error())
		}
	}

	response.Success(c, account)
}

// StartOAuthLogin initiates OAuth login via clandes's ClaudeAuthService.
// POST /api/v1/admin/clandes/oauth/start
func (h *ClandesHandler) StartOAuthLogin(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	var req struct {
		RedirectURI string `json:"redirect_uri" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	authURL, sessionID, err := h.client.StartOAuthLogin(c.Request.Context(), req.RedirectURI)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"auth_url": authURL, "session_id": sessionID})
}

// CompleteOAuthLogin exchanges the code for tokens and creates a clandes account.
// POST /api/v1/admin/clandes/oauth/callback
func (h *ClandesHandler) CompleteOAuthLogin(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	var req struct {
		SessionID   string `json:"session_id" binding:"required"`
		Code        string `json:"code" binding:"required"`
		CallbackURL string `json:"callback_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	result, err := h.client.CompleteOAuthLogin(c.Request.Context(), req.SessionID, req.Code, req.CallbackURL)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Create the account locally with clandes flag
	name := "OAuth"
	if result.Email != "" {
		name = result.Email
	}
	account := &service.Account{
		Name:     name,
		Platform: service.PlatformAnthropic,
		Type:     service.AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":  result.AccessToken,
			"refresh_token": result.RefreshToken,
		},
		Extra:       map[string]any{"clandes": true},
		Concurrency: 1,
		Status:      "active",
	}
	if err := h.accountRepo.Create(c.Request.Context(), account); err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("create account: %v", err))
		return
	}

	// Register to clandes
	if err := service.RegisterSingleAccountToClandes(c.Request.Context(), h.client, account); err != nil {
		c.Writer.Header().Set("X-Clandes-Sync-Warning", err.Error())
	}

	response.Success(c, account)
}

// DeleteAccount removes a clandes account from both sub2api and clandes.
// DELETE /api/v1/admin/clandes/accounts/:id
func (h *ClandesHandler) DeleteAccount(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}

	// Remove from clandes first (best-effort)
	if h.client != nil {
		_ = service.RemoveAccountFromClandes(c.Request.Context(), h.client, id)
	}

	if err := h.accountRepo.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, http.StatusInternalServerError, fmt.Sprintf("delete account: %v", err))
		return
	}

	response.Success(c, gin.H{"message": "account deleted"})
}

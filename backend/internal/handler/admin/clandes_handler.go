package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ClandesHandler exposes clandes integration status and control to the admin UI.
type ClandesHandler struct {
	client       *service.ClandesClient // may be nil when clandes.enabled=false
	accountRepo  service.AccountRepository
	proxyRepo    service.ProxyRepository
	adminService service.AdminService
}

// NewClandesHandler creates a ClandesHandler.
func NewClandesHandler(client *service.ClandesClient, accountRepo service.AccountRepository, proxyRepo service.ProxyRepository, adminService service.AdminService) *ClandesHandler {
	return &ClandesHandler{
		client:       client,
		accountRepo:  accountRepo,
		proxyRepo:    proxyRepo,
		adminService: adminService,
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

// StartOAuthLogin initiates OAuth login via clandes.
// Supports both Claude (platform="anthropic", default) and Codex (platform="openai").
// POST /api/v1/admin/clandes/oauth/start
func (h *ClandesHandler) StartOAuthLogin(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	var req struct {
		RedirectURI string `json:"redirect_uri" binding:"required"`
		ProxyID     *int64 `json:"proxy_id"`
		Platform    string `json:"platform"` // "anthropic" (default) or "openai"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var proxyURL string
	if req.ProxyID != nil {
		if p, err := h.proxyRepo.GetByID(c.Request.Context(), *req.ProxyID); err == nil && p != nil {
			proxyURL = service.BuildProxyURL(p)
		}
	}

	platform := req.Platform
	if platform == "" {
		platform = service.PlatformAnthropic
	}

	var authURL, sessionID string
	var err error
	if platform == service.PlatformOpenAI {
		authURL, sessionID, err = h.client.StartCodexOAuthLogin(c.Request.Context(), req.RedirectURI, proxyURL)
	} else {
		authURL, sessionID, err = h.client.StartOAuthLogin(c.Request.Context(), req.RedirectURI, proxyURL)
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{"auth_url": authURL, "session_id": sessionID})
}

// CompleteOAuthLogin exchanges the code for tokens via clandes.
// Supports both Claude (platform="anthropic", default) and Codex (platform="openai").
// POST /api/v1/admin/clandes/oauth/exchange
func (h *ClandesHandler) CompleteOAuthLogin(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	var req struct {
		SessionID   string `json:"session_id" binding:"required"`
		Code        string `json:"code" binding:"required"`
		CallbackURL string `json:"callback_url"` // required for Claude, unused for Codex
		Platform    string `json:"platform"`      // "anthropic" (default) or "openai"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	platform := req.Platform
	if platform == "" {
		platform = service.PlatformAnthropic
	}

	if platform == service.PlatformOpenAI {
		result, err := h.client.CompleteCodexOAuthLogin(c.Request.Context(), req.SessionID, req.Code)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		response.Success(c, gin.H{
			"account_id":         result.AccountID,
			"access_token":       result.AccessToken,
			"refresh_token":      result.RefreshToken,
			"id_token":           result.IDToken,
			"expires_in":         result.ExpiresIn,
			"chatgpt_account_id": result.ChatGPTAccountID,
			"email":              result.Email,
			"plan_type":          result.PlanType,
		})
		return
	}

	// Claude (default)
	if req.CallbackURL == "" {
		response.Error(c, http.StatusBadRequest, "callback_url is required")
		return
	}
	result, err := h.client.CompleteOAuthLogin(c.Request.Context(), req.SessionID, req.Code, req.CallbackURL)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"expires_in":    result.ExpiresIn,
		"email":         result.Email,
		"org_uuid":      result.OrganizationID,
	})
}

// RefreshCodexAccountToken refreshes an OpenAI/Codex account's tokens via clandes
// and writes the refreshed credentials back to sub2api.
// POST /api/v1/admin/clandes/accounts/:id/refresh
func (h *ClandesHandler) RefreshCodexAccountToken(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeOAuth {
		response.Error(c, http.StatusBadRequest, "account is not an OpenAI OAuth account")
		return
	}

	result, err := h.client.RefreshCodexAccountToken(c.Request.Context(), strconv.FormatInt(accountID, 10))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	newCreds := make(map[string]interface{}, len(account.Credentials)+4)
	for k, v := range account.Credentials {
		newCreds[k] = v
	}
	newCreds["access_token"] = result.AccessToken
	if result.RefreshToken != "" {
		newCreds["refresh_token"] = result.RefreshToken
	}
	if result.IDToken != "" {
		newCreds["id_token"] = result.IDToken
	}
	if result.ExpiresIn > 0 {
		newCreds["expires_in"] = result.ExpiresIn
	}

	updated, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Credentials: newCreds,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"account_id": updated.ID,
		"expires_in": result.ExpiresIn,
	})
}

// GetCodexProfile fetches a Codex account's profile via clandes CodexQueryService.
// GET /api/v1/admin/clandes/accounts/:id/profile
func (h *ClandesHandler) GetCodexProfile(c *gin.Context) {
	if h.client == nil {
		response.Error(c, http.StatusServiceUnavailable, "clandes integration is not enabled")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid account id")
		return
	}
	profile, err := h.client.GetCodexProfile(c.Request.Context(), strconv.FormatInt(accountID, 10))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, gin.H{
		"account_id":         profile.AccountID,
		"chatgpt_account_id": profile.ChatGPTAccountID,
		"email":              profile.Email,
		"plan_type":          profile.PlanType,
	})
}

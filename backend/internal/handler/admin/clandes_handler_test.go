//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestClandesHandler_GetStatus_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClandesHandler(nil, nil, nil, nil)

	router := gin.New()
	router.GET("/status", h.GetStatus)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data service.ClandesStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.Data.Enabled)
	require.False(t, resp.Data.Connected)
	require.Empty(t, resp.Data.Addr)
}

func TestClandesHandler_SyncAccounts_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClandesHandler(nil, nil, nil, nil)

	router := gin.New()
	router.POST("/sync", h.SyncAccounts)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestClandesHandler_StartOAuthLogin_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClandesHandler(nil, nil, nil, nil)

	router := gin.New()
	router.POST("/oauth/start", h.StartOAuthLogin)

	body, _ := json.Marshal(map[string]any{
		"redirect_uri": "https://example.com",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestClandesHandler_CompleteOAuthLogin_NilClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClandesHandler(nil, nil, nil, nil)

	router := gin.New()
	router.POST("/oauth/exchange", h.CompleteOAuthLogin)

	body, _ := json.Marshal(map[string]any{
		"session_id":   "sess-1",
		"code":         "auth-code",
		"callback_url": "https://example.com/callback",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestClandesHandler_CompleteOAuthLogin_NilClient_BadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewClandesHandler(nil, nil, nil, nil)

	router := gin.New()
	router.POST("/oauth/exchange", h.CompleteOAuthLogin)

	// Missing required fields — but nil client check comes first
	body, _ := json.Marshal(map[string]any{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/exchange", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

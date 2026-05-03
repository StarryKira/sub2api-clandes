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

func TestAccountHandler_Create_ClandesExtraForwarded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(
		adminSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // clandesClient is nil — no auto-register, but extra should still be forwarded
	)

	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	body := map[string]any{
		"name":     "clandes-oauth-1",
		"platform": "anthropic",
		"type":     "oauth",
		"credentials": map[string]any{
			"access_token":  "at-test",
			"refresh_token": "rt-test",
		},
		"extra": map[string]any{
			"clandes": true,
		},
		"concurrency": 1,
		"priority":    1,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)

	created := adminSvc.createdAccounts[0]
	require.Equal(t, "anthropic", created.Platform)
	require.Equal(t, "oauth", created.Type)
	require.NotNil(t, created.Extra)
	require.Equal(t, true, created.Extra["clandes"])
}

func TestAccountHandler_Create_ClandesWithProxyID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	adminSvc := newStubAdminService()
	handler := NewAccountHandler(
		adminSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, // clandesClient nil — auto-register won't fire, but input should carry proxy_id
	)

	router := gin.New()
	router.POST("/api/v1/admin/accounts", handler.Create)

	proxyID := int64(4) // matches the stub proxy
	body := map[string]any{
		"name":     "clandes-proxy",
		"platform": "anthropic",
		"type":     "oauth",
		"credentials": map[string]any{
			"access_token":  "at-test",
			"refresh_token": "rt-test",
		},
		"extra": map[string]any{
			"clandes": true,
		},
		"proxy_id":    proxyID,
		"concurrency": 1,
		"priority":    1,
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, adminSvc.createdAccounts, 1)

	created := adminSvc.createdAccounts[0]
	require.NotNil(t, created.ProxyID)
	require.Equal(t, proxyID, *created.ProxyID)
	require.Equal(t, true, created.Extra["clandes"])
}

func TestIsClandesAccount_Inline(t *testing.T) {
	// Verify IsClandesAccount works with various extra configurations
	tests := []struct {
		name   string
		extra  map[string]any
		expect bool
	}{
		{"nil", nil, false},
		{"empty", map[string]any{}, false},
		{"true", map[string]any{"clandes": true}, true},
		{"false", map[string]any{"clandes": false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acc := &service.Account{Extra: tt.extra}
			require.Equal(t, tt.expect, service.IsClandesAccount(acc))
		})
	}
}

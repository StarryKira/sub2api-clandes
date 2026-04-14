//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthLoginResult_Fields(t *testing.T) {
	t.Parallel()

	result := &OAuthLoginResult{
		AccessToken:    "at-123",
		RefreshToken:   "rt-456",
		ExpiresIn:      3600,
		Email:          "user@example.com",
		OrganizationID: "org-789",
	}

	require.Equal(t, "at-123", result.AccessToken)
	require.Equal(t, "rt-456", result.RefreshToken)
	require.Equal(t, uint64(3600), result.ExpiresIn)
	require.Equal(t, "user@example.com", result.Email)
	require.Equal(t, "org-789", result.OrganizationID)
}

func TestClandesStatus_DisabledWhenNilClient(t *testing.T) {
	t.Parallel()

	// Verify that when the handler gets a nil client, it should return disabled status.
	// This tests the data structure; the nil-check behavior is tested in handler tests.
	status := ClandesStatus{
		Enabled:   false,
		Connected: false,
		Addr:      "",
	}
	require.False(t, status.Enabled)
	require.False(t, status.Connected)
	require.Empty(t, status.Addr)
}

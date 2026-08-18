package tests

import (
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/config"
)

func TestLogoutRevokesJWTSession(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodPost, "/api/v1/auth/logout", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestRevokeAllSessions(t *testing.T) {
	router := testRouter(t)
	first := login(t, router, "admin", "admin123456")
	second := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodPost, "/api/v1/auth/sessions/revoke-all", first, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("revoke all status=%d body=%s", resp.Code, resp.Body.String())
	}
	for _, token := range []string{first, second} {
		resp = request(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("token should be revoked, status=%d body=%s", resp.Code, resp.Body.String())
		}
	}
}

func TestPasswordChangeInvalidatesExistingToken(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	resp := request(t, router, http.MethodPost, "/api/v1/auth/change-password", token, map[string]string{
		"oldPassword": "admin123456",
		"newPassword": "newpass123",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("change password status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/auth/me", token, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("old token status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestLoginRateLimitIncludesUnknownAccountsAndIP(t *testing.T) {
	router := testRouterWithConfig(t, config.Config{Security: config.SecurityConfig{
		LoginMaxAttempts:   2,
		LoginWindowMinutes: 15,
		LoginBlockMinutes:  15,
	}})
	for i := 0; i < 2; i++ {
		resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"username": "missing",
			"password": "wrong",
		})
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d body=%s", i+1, resp.Code, resp.Body.String())
		}
	}
	resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "another",
		"password": "wrong",
	})
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status=%d body=%s", resp.Code, resp.Body.String())
	}
}

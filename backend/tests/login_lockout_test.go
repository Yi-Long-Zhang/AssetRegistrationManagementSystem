package tests

import (
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestLoginLockout 验证登录暴力破解防护：连续 5 次失败后锁定，锁定期间正确密码也被拒。
func TestLoginLockout(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "lockuser", "锁定用户", model.RoleApplicant)

	// 连续 5 次错误密码
	for i := 0; i < 5; i++ {
		resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"username": "lockuser",
			"password": "wrong-password",
		})
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d body=%s", i+1, resp.Code, resp.Body.String())
		}
	}

	// 锁定后即使正确密码也应被拒
	resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "lockuser",
		"password": "user123456",
	})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("locked user should be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}
}

// TestLoginFailureResetsOnSuccess 验证失败计数在成功登录后清零（不累积）。
func TestLoginFailureResetsOnSuccess(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "resetuser", "清零用户", model.RoleApplicant)

	// 4 次失败（未达锁定阈值）
	for i := 0; i < 4; i++ {
		request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"username": "resetuser",
			"password": "wrong-password",
		})
	}
	// 成功登录，计数清零
	if resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "resetuser",
		"password": "user123456",
	}); resp.Code != http.StatusOK {
		t.Fatalf("success login should pass, got %d body=%s", resp.Code, resp.Body.String())
	}
	// 再失败 4 次不应触发锁定（计数已清零）
	for i := 0; i < 4; i++ {
		request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
			"username": "resetuser",
			"password": "wrong-password",
		})
	}
	// 第 5 次失败才触发锁定，正确密码此时应被拒
	request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "resetuser",
		"password": "wrong-password",
	})
	resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "resetuser",
		"password": "user123456",
	})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("after cumulative 5 failures user should be locked, got %d", resp.Code)
	}
}

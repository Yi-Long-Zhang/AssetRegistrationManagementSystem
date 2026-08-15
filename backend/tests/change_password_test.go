package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestChangePassword 验证改密接口：校验旧密码、新密码长度，成功后清除强制改密标记。
func TestChangePassword(t *testing.T) {
	router := testRouter(t)

	// 初始 admin 登录应带 mustChangePassword=true
	resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "admin",
		"password": "admin123456",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", resp.Code, resp.Body.String())
	}
	var loginData struct {
		Token string `json:"token"`
		User  struct {
			MustChangePassword bool `json:"mustChangePassword"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &loginData); err != nil {
		t.Fatal(err)
	}
	if !loginData.User.MustChangePassword {
		t.Fatal("seed admin should require password change on first login")
	}
	token := loginData.Token

	// 错误旧密码
	resp = request(t, router, http.MethodPost, "/api/v1/auth/change-password", token, map[string]string{
		"oldPassword": "wrong",
		"newPassword": "newpass123",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("wrong old password should be 400, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 新密码过短
	resp = request(t, router, http.MethodPost, "/api/v1/auth/change-password", token, map[string]string{
		"oldPassword": "admin123456",
		"newPassword": "short",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("short new password should be 400, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 正确改密
	resp = request(t, router, http.MethodPost, "/api/v1/auth/change-password", token, map[string]string{
		"oldPassword": "admin123456",
		"newPassword": "newpass123",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("change password status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 用新密码重新登录，mustChangePassword 应为 false
	resp = request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": "admin",
		"password": "newpass123",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("relogin status=%d body=%s", resp.Code, resp.Body.String())
	}
	loginData = struct {
		Token string `json:"token"`
		User  struct {
			MustChangePassword bool `json:"mustChangePassword"`
		} `json:"user"`
	}{}
	if err := json.Unmarshal(resp.Body.Bytes(), &loginData); err != nil {
		t.Fatal(err)
	}
	if loginData.User.MustChangePassword {
		t.Fatal("mustChangePassword should be false after password change")
	}
}

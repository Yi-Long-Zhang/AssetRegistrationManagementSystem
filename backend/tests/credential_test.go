package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestCredentialVault 验证凭据托管：创建加密存储、列表不含明文、查看解密、更新、删除与越权拦截。
func TestCredentialVault(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 创建凭据（明文应被 AES-GCM 加密，响应不得泄漏明文或密文）
	resp := request(t, router, http.MethodPost, "/api/v1/credentials", adminToken, map[string]interface{}{
		"name":     "root 登录",
		"username": "root",
		"type":     "ssh",
		"secret":   "my-secret-password",
		"remark":   "测试凭据",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create credential status=%d body=%s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("my-secret-password")) {
		t.Fatal("create response leaked plaintext secret")
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("encryptedSecret")) {
		t.Fatal("create response leaked encrypted secret field")
	}
	var created struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatal("expected credential id")
	}

	// 列表不含明文
	resp = request(t, router, http.MethodGet, "/api/v1/credentials", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list credentials status=%d body=%s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("my-secret-password")) {
		t.Fatal("list leaked plaintext secret")
	}

	// 查看明文（AES-GCM 往返）
	resp = request(t, router, http.MethodPost, "/api/v1/credentials/"+itoa(created.ID)+"/reveal", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("reveal credential status=%d body=%s", resp.Code, resp.Body.String())
	}
	var revealed struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &revealed); err != nil {
		t.Fatal(err)
	}
	if revealed.Secret != "my-secret-password" {
		t.Fatalf("revealed secret mismatch, got %q", revealed.Secret)
	}

	// 更新 secret
	resp = request(t, router, http.MethodPut, "/api/v1/credentials/"+itoa(created.ID), adminToken, map[string]interface{}{
		"name":     "root 登录",
		"username": "root",
		"type":     "ssh",
		"secret":   "new-secret-password",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update credential status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/credentials/"+itoa(created.ID)+"/reveal", adminToken, nil)
	if err := json.Unmarshal(resp.Body.Bytes(), &revealed); err != nil {
		t.Fatal(err)
	}
	if revealed.Secret != "new-secret-password" {
		t.Fatalf("updated secret mismatch, got %q", revealed.Secret)
	}

	// 删除
	resp = request(t, router, http.MethodDelete, "/api/v1/credentials/"+itoa(created.ID), adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("delete credential status=%d body=%s", resp.Code, resp.Body.String())
	}

	// applicant 越权
	createTestUser(t, router, adminToken, "cred-guard", "守卫", model.RoleApplicant)
	guardToken := login(t, router, "cred-guard", "user123456")
	resp = request(t, router, http.MethodGet, "/api/v1/credentials", guardToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant should be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}
}

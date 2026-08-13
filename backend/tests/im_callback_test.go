package tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
)

// TestIMCallbackConfigAndDingTalkVerify 验证回调验签配置与钉钉验签分派。
func TestIMCallbackConfigAndDingTalkVerify(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 保存钉钉回调验签配置
	resp := request(t, router, http.MethodPut, "/api/v1/settings/im/callback", token, map[string]interface{}{
		"enabled":   true,
		"platform":  "dingtalk",
		"appSecret": "ding-app-secret",
		"corpId":    "",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("save callback config status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 读取配置（敏感字段不回显）
	resp = request(t, router, http.MethodGet, "/api/v1/settings/im/callback", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get callback config status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !contains(resp.Body.String(), `"hasAppSecret":true`) {
		t.Fatalf("expect hasAppSecret=true, body=%s", resp.Body.String())
	}
	if contains(resp.Body.String(), "ding-app-secret") {
		t.Fatal("appSecret must not be echoed in plaintext")
	}

	// 构造正确钉钉签名
	secret := "ding-app-secret"
	timestamp := "1723456789000"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	body := map[string]interface{}{
		"action":   "approve",
		"ticketId": 999,
		"imUserId": "u1",
		"platform": "dingtalk",
	}
	// 正确签名 → 通过验签，走到用户绑定检查（403 未绑定，说明验签已通过）
	resp = request(t, router, http.MethodPost, "/api/v1/im/callback?signature="+sign+"&timestamp="+timestamp, "", body)
	if resp.Code != http.StatusForbidden || !contains(resp.Body.String(), "未绑定") {
		t.Fatalf("valid sign should reach binding check, status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 错误签名 → 验签失败
	resp = request(t, router, http.MethodPost, "/api/v1/im/callback?signature=bad&timestamp="+timestamp, "", body)
	if resp.Code != http.StatusForbidden || !contains(resp.Body.String(), "验签失败") {
		t.Fatalf("invalid sign should be rejected, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

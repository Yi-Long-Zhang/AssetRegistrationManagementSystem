package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestIPSegmentUsage 验证 IP 网段管理：CRUD、使用统计与冲突识别。
func TestIPSegmentUsage(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 创建网段
	resp := request(t, router, http.MethodPost, "/api/v1/ip-segments", adminToken, map[string]interface{}{
		"name":        "生产网段 A",
		"cidr":        "10.0.0.0/24",
		"description": "生产环境",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create segment status=%d body=%s", resp.Code, resp.Body.String())
	}
	var seg model.IPSegment
	if err := json.Unmarshal(resp.Body.Bytes(), &seg); err != nil {
		t.Fatal(err)
	}

	// 非法 CIDR 拒绝
	resp = request(t, router, http.MethodPost, "/api/v1/ip-segments", adminToken, map[string]interface{}{
		"name": "非法网段",
		"cidr": "not-a-cidr",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("illegal cidr should be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 在网段内创建两台资产
	createTestAsset(t, router, adminToken, "10.0.0.5", "pool-a", "owner")
	createTestAsset(t, router, adminToken, "10.0.0.10", "pool-b", "owner")

	// 使用统计
	resp = request(t, router, http.MethodGet, "/api/v1/ip-segments/"+itoa(seg.ID)+"/usage", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("usage status=%d body=%s", resp.Code, resp.Body.String())
	}
	var usage struct {
		Total     int64 `json:"total"`
		Used      int   `json:"used"`
		Available int64 `json:"available"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Total != 254 || usage.Used != 2 || usage.Available != 252 {
		t.Fatalf("unexpected usage: total=%d used=%d available=%d", usage.Total, usage.Used, usage.Available)
	}

	// 通过关联 IP（additionalIPs）构造占用冲突：资产 C 的关联 IP 与资产 A 主 IP 相同
	resp = request(t, router, http.MethodPost, "/api/v1/assets", adminToken, map[string]interface{}{
		"ip":            "10.0.0.20",
		"hostname":      "pool-c",
		"owner":         "owner",
		"status":        model.AssetStatusInUse,
		"additionalIPs": "10.0.0.5",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create conflict asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/ip-segments/"+itoa(seg.ID)+"/usage", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("usage after conflict status=%d", resp.Code)
	}
	var usage2 struct {
		Used      int `json:"used"`
		Conflicts []struct {
			IP    string `json:"ip"`
			Count int    `json:"count"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &usage2); err != nil {
		t.Fatal(err)
	}
	if usage2.Used != 3 {
		t.Fatalf("expected used=3 unique IPs, got %d", usage2.Used)
	}
	if len(usage2.Conflicts) != 1 || usage2.Conflicts[0].IP != "10.0.0.5" || usage2.Conflicts[0].Count != 2 {
		t.Fatalf("expected conflict on 10.0.0.5 count=2, got %+v", usage2.Conflicts)
	}
}

// TestIPSegmentRoleGuard 验证普通用户无权管理网段。
func TestIPSegmentRoleGuard(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")
	createTestUser(t, router, adminToken, "guard-user", "守卫", model.RoleApplicant)
	userToken := login(t, router, "guard-user", "user123456")

	resp := request(t, router, http.MethodPost, "/api/v1/ip-segments", userToken, map[string]interface{}{
		"name": "越权网段",
		"cidr": "192.168.0.0/24",
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant should be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}
}

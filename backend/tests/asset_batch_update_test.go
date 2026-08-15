package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestBatchUpdateAssets 验证批量编辑接口：白名单字段更新、快照与审计落库、越权字段拦截、角色权限。
func TestBatchUpdateAssets(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	asset1 := createTestAsset(t, router, adminToken, "10.1.0.1", "batch-a", "old-owner")
	asset2 := createTestAsset(t, router, adminToken, "10.1.0.2", "batch-b", "old-owner")

	// 正常批量更新（白名单字段：含 camelCase 字段与日期字段）
	resp := request(t, router, http.MethodPost, "/api/v1/assets/batch-update", adminToken, map[string]interface{}{
		"ids": []uint{asset1, asset2},
		"fields": map[string]interface{}{
			"owner":              "new-owner",
			"department":         "运维部",
			"status":             string(model.AssetStatusMaintenance),
			"businessSystem":     "ERP",
			"warrantyExpireDate": "2027-12-31",
		},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("batch update status=%d body=%s", resp.Code, resp.Body.String())
	}
	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Updated != 2 {
		t.Fatalf("expected updated=2, got %d", result.Updated)
	}

	// 校验资产已更新（含 camelCase 列名映射与日期解析）
	asset := getAsset(t, router, adminToken, asset1)
	if asset.Owner != "new-owner" || asset.Department != "运维部" || asset.Status != model.AssetStatusMaintenance {
		t.Fatalf("asset not updated correctly: %+v", asset)
	}
	if asset.BusinessSystem != "ERP" {
		t.Fatalf("expected businessSystem=ERP, got %q", asset.BusinessSystem)
	}
	if asset.WarrantyExpireDate == nil || asset.WarrantyExpireDate.Format("2006-01-02") != "2027-12-31" {
		t.Fatalf("expected warrantyExpireDate=2027-12-31, got %v", asset.WarrantyExpireDate)
	}

	// 非法日期格式应被拦截
	resp = request(t, router, http.MethodPost, "/api/v1/assets/batch-update", adminToken, map[string]interface{}{
		"ids":    []uint{asset1},
		"fields": map[string]interface{}{"warrantyExpireDate": "not-a-date"},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("illegal date should be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 快照已生成（变更历史非空）
	if resp := request(t, router, http.MethodGet, "/api/v1/assets/"+itoa(asset1)+"/history", adminToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("asset history status=%d", resp.Code)
	} else {
		var history []model.AssetSnapshot
		if err := json.Unmarshal(resp.Body.Bytes(), &history); err != nil {
			t.Fatal(err)
		}
		if len(history) == 0 {
			t.Fatal("expected asset snapshot history after batch update")
		}
	}

	// 越权字段（关键标识）应被拦截
	resp = request(t, router, http.MethodPost, "/api/v1/assets/batch-update", adminToken, map[string]interface{}{
		"ids":    []uint{asset1},
		"fields": map[string]interface{}{"ip": "10.99.99.99"},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("illegal field should be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 非法状态值应被拦截
	resp = request(t, router, http.MethodPost, "/api/v1/assets/batch-update", adminToken, map[string]interface{}{
		"ids":    []uint{asset1},
		"fields": map[string]interface{}{"status": "flying"},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("illegal status should be rejected, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 空字段集应被拦截
	resp = request(t, router, http.MethodPost, "/api/v1/assets/batch-update", adminToken, map[string]interface{}{
		"ids":    []uint{asset1},
		"fields": map[string]interface{}{},
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("empty fields should be rejected, got %d", resp.Code)
	}
}

// TestBatchUpdateAssetsRoleGuard 验证非 admin/asset_manager 无权限调用批量编辑。
func TestBatchUpdateAssetsRoleGuard(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")
	asset1 := createTestAsset(t, router, adminToken, "10.2.0.1", "guard-a", "guard-owner")

	// 创建 applicant 用户
	resp := request(t, router, http.MethodPost, "/api/v1/users", adminToken, map[string]interface{}{
		"username": "guard-user",
		"name":     "守卫",
		"password": "user123456",
		"role":     model.RoleApplicant,
		"status":   "active",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", resp.Code, resp.Body.String())
	}
	userToken := login(t, router, "guard-user", "user123456")

	resp = request(t, router, http.MethodPost, "/api/v1/assets/batch-update", userToken, map[string]interface{}{
		"ids":    []uint{asset1},
		"fields": map[string]interface{}{"remark": "hack"},
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant should be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func getAsset(t *testing.T, router http.Handler, token string, id uint) model.Asset {
	t.Helper()
	resp := request(t, router, http.MethodGet, "/api/v1/assets/"+itoa(id), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	var asset model.Asset
	if err := json.Unmarshal(resp.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}
	return asset
}

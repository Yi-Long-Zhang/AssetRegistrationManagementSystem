package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestAssetDataScopeByRole 验证资产数据级权限：
// admin/asset_manager 看全量；普通用户（applicant/approver）仅见 owner
// 匹配自己姓名或用户名的资产，且无权查看他人资产详情。
func TestAssetDataScopeByRole(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 创建普通用户 zhangsan（applicant 角色）
	resp := request(t, router, http.MethodPost, "/api/v1/users", adminToken, map[string]interface{}{
		"username": "zhangsan",
		"name":     "张三",
		"password": "user123456",
		"role":     model.RoleApplicant,
		"status":   "active",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", resp.Code, resp.Body.String())
	}

	// admin 创建两个资产：一个负责人为 zhangsan，另一个为他人
	ownAsset := createTestAsset(t, router, adminToken, "10.0.0.1", "srv-a", "zhangsan")
	otherAsset := createTestAsset(t, router, adminToken, "10.0.0.2", "srv-b", "lisi")
	// 无负责人资产：普通用户同样不可见
	createTestAsset(t, router, adminToken, "10.0.0.3", "srv-c", "")

	// admin 应看到全部资产
	adminList := listAssetsTotal(t, router, adminToken)
	if adminList != 3 {
		t.Fatalf("admin should see 3 assets, got %d", adminList)
	}

	// 普通用户登录
	userToken := login(t, router, "zhangsan", "user123456")

	// 列表仅返回自己负责的资产
	var userList struct {
		Items []model.Asset `json:"items"`
		Total int64         `json:"total"`
	}
	resp = request(t, router, http.MethodGet, "/api/v1/assets?page=1&pageSize=50", userToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list assets status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &userList); err != nil {
		t.Fatal(err)
	}
	if userList.Total != 1 || len(userList.Items) != 1 || userList.Items[0].ID != ownAsset {
		t.Fatalf("user should only see own asset, total=%d items=%d firstID=%d want=%d",
			userList.Total, len(userList.Items), userList.Items[0].ID, ownAsset)
	}

	// 详情：自己的 200，他人的 403
	if resp := request(t, router, http.MethodGet, "/api/v1/assets/"+itoa(ownAsset), userToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("get own asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	if resp := request(t, router, http.MethodGet, "/api/v1/assets/"+itoa(otherAsset), userToken, nil); resp.Code != http.StatusForbidden {
		t.Fatalf("get others asset should be 403, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 资产管理员也应看全量
	managerToken := login(t, router, "admin", "admin123456") // admin 即 asset_manager 权限
	if total := listAssetsTotal(t, router, managerToken); total != 3 {
		t.Fatalf("asset manager should see 3 assets, got %d", total)
	}
}

func createTestAsset(t *testing.T, router http.Handler, token, ip, hostname, owner string) uint {
	t.Helper()
	resp := request(t, router, http.MethodPost, "/api/v1/assets", token, map[string]interface{}{
		"ip":       ip,
		"hostname": hostname,
		"owner":    owner,
		"status":   model.AssetStatusInUse,
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	var asset model.Asset
	if err := json.Unmarshal(resp.Body.Bytes(), &asset); err != nil {
		t.Fatal(err)
	}
	return asset.ID
}

func listAssetsTotal(t *testing.T, router http.Handler, token string) int64 {
	t.Helper()
	resp := request(t, router, http.MethodGet, "/api/v1/assets?page=1&pageSize=1", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list assets status=%d body=%s", resp.Code, resp.Body.String())
	}
	var data struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data.Total
}

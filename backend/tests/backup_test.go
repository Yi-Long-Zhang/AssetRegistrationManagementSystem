package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestDatabaseBackup 验证数据库备份：创建/列表/下载/恢复标记，及越权拦截。
func TestDatabaseBackup(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 创建备份
	resp := request(t, router, http.MethodPost, "/api/v1/backups", adminToken, nil)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create backup status=%d body=%s", resp.Code, resp.Body.String())
	}
	var created struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Name == "" {
		t.Fatal("expected backup name")
	}

	// 列表
	resp = request(t, router, http.MethodGet, "/api/v1/backups", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list backups status=%d body=%s", resp.Code, resp.Body.String())
	}
	var list struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != created.Name {
		t.Fatalf("expected 1 backup %q, got %+v", created.Name, list.Items)
	}

	// 下载（应返回 SQLite 文件头 "SQLite format 3"）
	resp = request(t, router, http.MethodGet, "/api/v1/backups/"+created.Name+"/download", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("download backup status=%d body=%s", resp.Code, resp.Body.String())
	}
	if len(resp.Body.Bytes()) < 16 || string(resp.Body.Bytes()[:15]) != "SQLite format 3" {
		t.Fatalf("downloaded file is not a SQLite database, head=%q", resp.Body.Bytes()[:minInt(15, len(resp.Body.Bytes()))])
	}

	// 恢复（标记待恢复）
	resp = request(t, router, http.MethodPost, "/api/v1/backups/"+created.Name+"/restore", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("restore backup status=%d body=%s", resp.Code, resp.Body.String())
	}

	// applicant 越权
	createTestUser(t, router, adminToken, "backup-guard", "守卫", model.RoleApplicant)
	guardToken := login(t, router, "backup-guard", "user123456")
	resp = request(t, router, http.MethodGet, "/api/v1/backups", guardToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant should be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

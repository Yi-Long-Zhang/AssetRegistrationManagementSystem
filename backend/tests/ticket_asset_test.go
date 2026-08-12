package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestTicketMultiAssetLink 验证工单多资产关联：创建带 assetIds 的工单、详情返回关联资产、更新关联列表。
func TestTicketMultiAssetLink(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 准备两个资产
	assetIDs := make([]uint, 0, 2)
	for i, ip := range []string{"10.0.1.11", "10.0.1.12"} {
		resp := request(t, router, http.MethodPost, "/api/v1/assets", token, map[string]interface{}{
			"assetNo":  "SRV-MULTI-00" + itoa(uint(i+1)),
			"hostname": "srv-multi-" + itoa(uint(i+1)),
			"ip":       ip,
			"status":   "in_use",
			"owner":    "平台组",
		})
		if resp.Code != http.StatusCreated {
			t.Fatalf("create asset %d status=%d body=%s", i, resp.Code, resp.Body.String())
		}
		var created struct {
			ID uint `json:"id"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		assetIDs = append(assetIDs, created.ID)
	}

	// 创建工单并关联两个资产
	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":     "maintenance",
		"title":    "多资产关联测试工单",
		"priority": "normal",
		"assetIds": assetIDs,
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}

	// 详情应返回两个关联资产
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticket.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var detail struct {
		Assets []model.TicketAsset `json:"assets"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Assets) != 2 {
		t.Fatalf("expect 2 linked assets, got %d", len(detail.Assets))
	}
	if detail.Assets[0].Asset.IP == "" {
		t.Fatal("linked asset should be preloaded with asset details")
	}

	// 更新工单为只关联第一个资产
	resp = request(t, router, http.MethodPut, "/api/v1/tickets/"+itoa(ticket.ID), token, map[string]interface{}{
		"type":     "maintenance",
		"title":    "多资产关联测试工单",
		"priority": "normal",
		"assetIds": assetIDs[:1],
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticket.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get ticket after update status=%d body=%s", resp.Code, resp.Body.String())
	}
	detail = struct {
		Assets []model.TicketAsset `json:"assets"`
	}{}
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Assets) != 1 {
		t.Fatalf("expect 1 linked asset after update, got %d", len(detail.Assets))
	}
}

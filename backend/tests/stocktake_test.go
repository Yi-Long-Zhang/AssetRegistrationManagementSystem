package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestStocktakeFlow 验证盘点单全流程：创建快照 → 核对 → 关闭 → 导出。
func TestStocktakeFlow(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 创建 2 个资产
	for i, ip := range []string{"10.0.2.11", "10.0.2.12"} {
		resp := request(t, router, http.MethodPost, "/api/v1/assets", token, map[string]interface{}{
			"assetNo":   "STK-00" + itoa(uint(i+1)),
			"hostname":  "stk-" + itoa(uint(i+1)),
			"ip":        ip,
			"assetType": "server",
			"status":    "in_use",
		})
		if resp.Code != http.StatusCreated {
			t.Fatalf("create asset %d status=%d body=%s", i, resp.Code, resp.Body.String())
		}
	}

	// 创建盘点单（按 assetType=server 过滤）
	resp := request(t, router, http.MethodPost, "/api/v1/stocktakes", token, map[string]interface{}{
		"name": "年度盘点", "assetType": "server",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create stocktake status=%d body=%s", resp.Code, resp.Body.String())
	}
	var task struct {
		ID    uint `json:"id"`
		Items []struct {
			ID     uint   `json:"id"`
			Result string `json:"result"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if len(task.Items) != 2 {
		t.Fatalf("expect 2 items snapshot, got %d", len(task.Items))
	}

	// 核对第一项为盘亏
	itemID := task.Items[0].ID
	resp = request(t, router, http.MethodPut, "/api/v1/stocktakes/"+itoa(task.ID)+"/items/"+itoa(itemID), token, map[string]interface{}{
		"result": "missing", "remark": "找不到该设备",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("check item status=%d body=%s", resp.Code, resp.Body.String())
	}
	// 核对第二项为一致
	resp = request(t, router, http.MethodPut, "/api/v1/stocktakes/"+itoa(task.ID)+"/items/"+itoa(task.Items[1].ID), token, map[string]interface{}{
		"result": "matched",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("check item2 status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 关闭盘点单
	resp = request(t, router, http.MethodPost, "/api/v1/stocktakes/"+itoa(task.ID)+"/close", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("close stocktake status=%d body=%s", resp.Code, resp.Body.String())
	}
	var summary struct {
		Total   int64 `json:"total"`
		Matched int64 `json:"matched"`
		Missing int64 `json:"missing"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Matched != 1 || summary.Missing != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	// 导出 CSV 含盘亏行
	resp = request(t, router, http.MethodGet, "/api/v1/stocktakes/"+itoa(task.ID)+"/export", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !contains(body, "盘亏") || !contains(body, "找不到该设备") {
		t.Fatalf("export csv missing content: %s", body)
	}
}

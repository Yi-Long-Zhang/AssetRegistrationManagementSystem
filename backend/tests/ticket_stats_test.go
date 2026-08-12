package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestTicketStatsAndExport 验证工单统计接口：分布聚合、SLA 达标率与 CSV 导出。
func TestTicketStatsAndExport(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 配置 maintenance 流程：单节点 + 审批时限 1 小时（用于 SLA 统计）
	approvalHours := 1
	resp := request(t, router, http.MethodPut, "/api/v1/workflows/maintenance", token, map[string]interface{}{
		"name":          "maintenance 流程",
		"enabled":       true,
		"approvalHours": &approvalHours,
		"nodes":         []map[string]interface{}{{"name": "审批节点", "approverIds": []uint{1}}},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("configure workflow status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 创建 3 张草稿工单（不同类型/优先级）
	for i, payload := range []map[string]interface{}{
		{"type": "maintenance", "title": "统计测试-维护", "priority": "high"},
		{"type": "asset_change", "title": "统计测试-变更", "priority": "normal"},
		{"type": "inspection", "title": "统计测试-巡检", "priority": "low"},
	} {
		resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, payload)
		if resp.Code != http.StatusCreated {
			t.Fatalf("create ticket %d status=%d body=%s", i, resp.Code, resp.Body.String())
		}
	}

	// 走完整流程关闭一张 maintenance 工单（含 SLA 审批截止 → 应达标）
	resp = request(t, router, http.MethodGet, "/api/v1/tickets?view=submitted", token, nil)
	var list struct {
		Items []struct {
			ID   uint   `json:"id"`
			Type string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	targetID := uint(0)
	for _, item := range list.Items {
		if item.Type == "maintenance" {
			targetID = item.ID
			break
		}
	}
	if targetID == 0 {
		t.Fatal("no maintenance ticket found for full flow")
	}
	for _, action := range []string{"submit", "approve", "start", "complete", "accept"} {
		body := map[string]interface{}{}
		if action == "complete" {
			body["result"] = "巡检完成，无异常"
		}
		if action == "accept" {
			body["acceptanceResult"] = "验收通过"
		}
		resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(targetID)+"/"+action, token, body)
		if resp.Code != http.StatusOK {
			t.Fatalf("action %s status=%d body=%s", action, resp.Code, resp.Body.String())
		}
	}

	// 统计接口
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/stats", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", resp.Code, resp.Body.String())
	}
	var stats struct {
		Total  int64 `json:"total"`
		ByType []struct {
			Label string
			Count int64
		} `json:"byType"`
		ByStatus []struct {
			Label string
			Count int64
		} `json:"byStatus"`
		MonthlyTrend []struct {
			Label string
			Count int64
		} `json:"monthlyTrend"`
		SLASummary struct {
			Total int64   `json:"total"`
			Met   int64   `json:"met"`
			Rate  float64 `json:"rate"`
		} `json:"slaSummary"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Total != 3 {
		t.Fatalf("expect total 3, got %d", stats.Total)
	}
	if len(stats.ByType) < 3 || len(stats.ByStatus) < 1 || len(stats.MonthlyTrend) != 12 {
		t.Fatalf("unexpected stats shape: total=%d byType=%d byStatus=%d trend=%d", stats.Total, len(stats.ByType), len(stats.ByStatus), len(stats.MonthlyTrend))
	}
	if stats.SLASummary.Total != 1 || stats.SLASummary.Met != 1 || stats.SLASummary.Rate != 100 {
		t.Fatalf("expect SLA 1/1 met rate=100, got %+v", stats.SLASummary)
	}

	// CSV 导出
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/stats/export", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("export status=%d body=%s", resp.Code, resp.Body.String())
	}
	body := string(resp.Body.Bytes())
	for _, term := range []string{"工单统计报表", "工单总数", "状态分布", "类型分布", "SLA 达标率", "月度趋势"} {
		if !contains(body, term) {
			t.Fatalf("export csv missing %q: %s", term, body)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

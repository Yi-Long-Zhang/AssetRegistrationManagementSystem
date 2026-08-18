package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestInspectionRuleCRUDAndTestRun 验证巡检规则 CRUD 与试运行生成巡检工单。
func TestInspectionRuleCRUDAndTestRun(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 新建规则
	createBody := map[string]interface{}{
		"name":        "机房每周巡检",
		"description": "检查机房设备运行状态",
		"frequency":   "weekly",
		"dayOfWeek":   5,
		"timeOfDay":   "10:00",
		"assigneeId":  1, // admin
		"enabled":     true,
	}
	resp := request(t, router, http.MethodPost, "/api/v1/inspection/rules", token, createBody)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create rule status=%d body=%s", resp.Code, resp.Body.String())
	}
	var rule struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &rule); err != nil {
		t.Fatal(err)
	}
	if rule.ID == 0 {
		t.Fatal("rule id should not be 0")
	}

	// 非法频率应被拒绝
	resp = request(t, router, http.MethodPost, "/api/v1/inspection/rules", token, map[string]interface{}{
		"name": "坏规则", "frequency": "hourly", "timeOfDay": "10:00", "assigneeId": 1,
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid frequency should be rejected, status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 试运行 → 生成巡检工单草稿
	resp = request(t, router, http.MethodPost, "/api/v1/inspection/rules/"+itoa(rule.ID)+"/test", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("test run status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket struct {
		ID     uint               `json:"id"`
		Type   model.TicketType   `json:"type"`
		Status model.TicketStatus `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Type != model.TicketTypeInspection || ticket.Status != model.TicketStatusDraft {
		t.Fatalf("unexpected ticket type/status: %+v", ticket)
	}

	// 列表应包含规则
	resp = request(t, router, http.MethodGet, "/api/v1/inspection/rules", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list rules status=%d body=%s", resp.Code, resp.Body.String())
	}
	var listed struct {
		Items []model.InspectionRule `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("expect 1 rule, got %d", len(listed.Items))
	}

	// 更新规则（改为每日 + 停用）
	resp = request(t, router, http.MethodPut, "/api/v1/inspection/rules/"+itoa(rule.ID), token, map[string]interface{}{
		"name": "机房每日巡检", "frequency": "daily", "timeOfDay": "08:00", "assigneeId": 1, "enabled": false,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update rule status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 删除规则
	resp = request(t, router, http.MethodDelete, "/api/v1/inspection/rules/"+itoa(rule.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("delete rule status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/inspection/rules", token, nil)
	var after struct {
		Items []model.InspectionRule `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 0 {
		t.Fatalf("expect 0 rules after delete, got %d", len(after.Items))
	}
}

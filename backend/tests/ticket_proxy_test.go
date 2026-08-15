package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestTicketProxyApproval 验证审批代理：代理设置后，代理人的待办可见性与审批权限包含被代理人的工单。
func TestTicketProxyApproval(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApplicant)
	approver1ID := createTestUser(t, router, adminToken, "approver1", "审批一", model.RoleApprover)
	approver2ID := createTestUser(t, router, adminToken, "approver2", "审批二", model.RoleApprover)

	// approver1 设置代理为 approver2
	resp := request(t, router, http.MethodPut, "/api/v1/users/"+itoa(approver1ID), adminToken, map[string]interface{}{
		"username":    "approver1",
		"name":        "审批一",
		"role":        model.RoleApprover,
		"status":      "active",
		"proxyUserId": approver2ID,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("update proxy status=%d body=%s", resp.Code, resp.Body.String())
	}

	configureWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"}, approver1ID)

	zhangsanToken := login(t, router, "zhangsan", "user123456")
	ticketID := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "代理审批测试", nil)
	if resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", zhangsanToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}

	// approver2（代理）待办可见被代理人工单
	approver2Token := login(t, router, "approver2", "user123456")
	resp = request(t, router, http.MethodGet, "/api/v1/tickets?view=todo&pageSize=50", approver2Token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list todo status=%d body=%s", resp.Code, resp.Body.String())
	}
	var todo struct {
		Items []struct {
			ID uint `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &todo); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range todo.Items {
		if item.ID == ticketID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("proxy approver should see proxied ticket %d in todo, got %+v", ticketID, todo.Items)
	}

	// 代理可审批
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/approve", approver2Token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("proxy approve status=%d body=%s", resp.Code, resp.Body.String())
	}
	if status := ticketStatus(t, router, zhangsanToken, ticketID); status != model.TicketStatusApproved {
		t.Fatalf("expected approved after proxy approve, got %s", status)
	}
}

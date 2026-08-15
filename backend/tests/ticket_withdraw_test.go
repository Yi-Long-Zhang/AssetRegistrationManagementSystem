package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestTicketWithdraw 验证工单撤回：审批中可撤回回草稿并清空流程快照，撤回后可重新提交；
// 非申请人无权撤回；非审批中状态不可撤回。
func TestTicketWithdraw(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 申请人 + 审批人
	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApplicant)
	approverID := createTestUser(t, router, adminToken, "approver1", "审批一", model.RoleApprover)

	// 配置流程（带审批人）
	configureWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"}, approverID)

	// 申请人创建并提交工单
	zhangsanToken := login(t, router, "zhangsan", "user123456")
	ticketID := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "撤回测试工单", nil)

	resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", zhangsanToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}
	if status := ticketStatus(t, router, zhangsanToken, ticketID); status != model.TicketStatusPendingApproval {
		t.Fatalf("expected pending_approval after submit, got %s", status)
	}

	// 申请人撤回 → 回草稿，流程快照清空
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/withdraw", zhangsanToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("withdraw status=%d body=%s", resp.Code, resp.Body.String())
	}
	if status := ticketStatus(t, router, zhangsanToken, ticketID); status != model.TicketStatusDraft {
		t.Fatalf("expected draft after withdraw, got %s", status)
	}
	if steps := ticketWorkflowStepCount(t, router, zhangsanToken, ticketID); steps != 0 {
		t.Fatalf("expected empty workflow steps after withdraw, got %d", steps)
	}

	// 撤回后可重新提交
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", zhangsanToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("resubmit status=%d body=%s", resp.Code, resp.Body.String())
	}
	if status := ticketStatus(t, router, zhangsanToken, ticketID); status != model.TicketStatusPendingApproval {
		t.Fatalf("expected pending_approval after resubmit, got %s", status)
	}

	// 非申请人无权撤回
	createTestUser(t, router, adminToken, "lisi", "李四", model.RoleApplicant)
	lisiToken := login(t, router, "lisi", "user123456")
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/withdraw", lisiToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("non-applicant withdraw should be 403, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 非审批中状态不可撤回（approve 后处于 approved，withdraw 应被状态机拒绝）
	approveToken := login(t, router, "approver1", "user123456")
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/approve", approveToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/withdraw", zhangsanToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("withdraw on approved status should be 403, got %d body=%s", resp.Code, resp.Body.String())
	}
}

func ticketStatus(t *testing.T, router http.Handler, token string, ticketID uint) model.TicketStatus {
	t.Helper()
	resp := request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticketID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var detail struct {
		Status model.TicketStatus `json:"status"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	return detail.Status
}

func ticketWorkflowStepCount(t *testing.T, router http.Handler, token string, ticketID uint) int {
	t.Helper()
	resp := request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticketID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var detail struct {
		WorkflowSteps []json.RawMessage `json:"workflowSteps"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	return len(detail.WorkflowSteps)
}

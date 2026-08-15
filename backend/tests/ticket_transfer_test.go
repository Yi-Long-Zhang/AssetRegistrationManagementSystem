package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestTicketTransfer 验证审批转交：当前审批人可转交，转交后原审批人无权、目标审批人可审批。
func TestTicketTransfer(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApplicant)
	approver1ID := createTestUser(t, router, adminToken, "approver1", "审批一", model.RoleApprover)
	approver2ID := createTestUser(t, router, adminToken, "approver2", "审批二", model.RoleApprover)

	configureWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"}, approver1ID)

	zhangsanToken := login(t, router, "zhangsan", "user123456")
	ticketID := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "转交测试工单", nil)
	if resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", zhangsanToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}

	approver1Token := login(t, router, "approver1", "user123456")
	approver2Token := login(t, router, "approver2", "user123456")

	// 转交给 approver2
	resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/transfer", approver1Token, map[string]interface{}{
		"toUserId": approver2ID,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("transfer status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 原审批人 approver1 再审批 → 403
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/approve", approver1Token, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("old approver should be forbidden, got %d body=%s", resp.Code, resp.Body.String())
	}

	// 新审批人 approver2 审批 → 200
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/approve", approver2Token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("new approver approve status=%d body=%s", resp.Code, resp.Body.String())
	}
}

// TestBatchApprove 验证批量审批：审批中的工单逐单生效，非审批中/越权计入 skipped。
func TestBatchApprove(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApplicant)
	approver1ID := createTestUser(t, router, adminToken, "approver1", "审批一", model.RoleApprover)
	createTestUser(t, router, adminToken, "approver2", "审批二", model.RoleApprover)

	configureWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"}, approver1ID)

	zhangsanToken := login(t, router, "zhangsan", "user123456")

	// 工单A：已提交（审批中）；工单B：已提交（审批中）；工单C：草稿（未提交）
	ticketA := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "批量审批A", nil)
	ticketB := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "批量审批B", nil)
	ticketC := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "批量审批C(草稿)", nil)
	for _, id := range []uint{ticketA, ticketB} {
		if resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(id)+"/submit", zhangsanToken, nil); resp.Code != http.StatusOK {
			t.Fatalf("submit ticket %d status=%d", id, resp.Code)
		}
	}

	approver1Token := login(t, router, "approver1", "user123456")

	// approver1 批量审批 [A, B, C]：A、B 成功，C（草稿）跳过
	resp := request(t, router, http.MethodPost, "/api/v1/tickets/batch-approve", approver1Token, map[string]interface{}{
		"ids":    []uint{ticketA, ticketB, ticketC},
		"remark": "批量通过",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("batch approve status=%d body=%s", resp.Code, resp.Body.String())
	}
	var result struct {
		Approved int    `json:"approved"`
		Skipped  []uint `json:"skipped"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Approved != 2 {
		t.Fatalf("expected approved=2, got %d", result.Approved)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != ticketC {
		t.Fatalf("expected skipped=[%d], got %v", ticketC, result.Skipped)
	}
	if status := ticketStatus(t, router, zhangsanToken, ticketA); status != model.TicketStatusApproved {
		t.Fatalf("expected ticketA approved, got %s", status)
	}

	// approver2 无权批量审批 approver1 的待办 → 全部跳过
	approver2Token := login(t, router, "approver2", "user123456")
	ticketD := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "批量审批D", nil)
	if resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketD)+"/submit", zhangsanToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("submit ticketD status=%d", resp.Code)
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/batch-approve", approver2Token, map[string]interface{}{
		"ids": []uint{ticketD},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("batch approve by non-approver status=%d body=%s", resp.Code, resp.Body.String())
	}
	result = struct {
		Approved int    `json:"approved"`
		Skipped  []uint `json:"skipped"`
	}{}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Approved != 0 || len(result.Skipped) != 1 {
		t.Fatalf("expected approved=0 skipped=1 for unauthorized approver, got %+v", result)
	}
}

// TestTransferAndBatchApproveRoleGuard 验证非审批角色（applicant）无权转交与批量审批。
func TestTransferAndBatchApproveRoleGuard(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApplicant)
	approverID := createTestUser(t, router, adminToken, "approver1", "审批一", model.RoleApprover)
	configureWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"}, approverID)

	zhangsanToken := login(t, router, "zhangsan", "user123456")
	ticketID := createTestTicket(t, router, zhangsanToken, string(model.TicketTypeMaintenance), "角色门禁测试", nil)
	if resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", zhangsanToken, nil); resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d", resp.Code)
	}

	// applicant 转交应 403
	resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/transfer", zhangsanToken, map[string]interface{}{
		"toUserId": approverID,
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant transfer should be 403, got %d body=%s", resp.Code, resp.Body.String())
	}

	// applicant 批量审批应 403
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/batch-approve", zhangsanToken, map[string]interface{}{
		"ids": []uint{ticketID},
	})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant batch approve should be 403, got %d body=%s", resp.Code, resp.Body.String())
	}
}

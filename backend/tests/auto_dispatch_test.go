package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

// TestAutoDispatchByAssetOwner 验证流程节点未配置审批人时，按工单关联资产的负责人自动分派。
func TestAutoDispatchByAssetOwner(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 创建 approver 用户（与资产 owner 匹配）
	zhangsanID := createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApprover)

	// 创建资产，owner = zhangsan
	assetID := createTestAsset(t, router, adminToken, "10.3.0.1", "dispatch-a", "zhangsan")

	// 配置工单流程：一个审批节点，无审批人
	configureEmptyWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"})

	// 创建工单并关联资产，提交审批
	ticketID := createTestTicket(t, router, adminToken, string(model.TicketTypeMaintenance), "自动分派测试", []uint{assetID})
	resp := request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}

	approverIDs := firstStepApproverIDs(t, router, adminToken, ticketID)
	if !containsUint(approverIDs, zhangsanID) {
		t.Fatalf("expected asset owner %d auto-dispatched, got %v", zhangsanID, approverIDs)
	}
}

// TestAutoDispatchFallbackToTypeApprover 验证无资产负责人命中时，回退类型默认审批人（TicketTypeApprover）。
func TestAutoDispatchFallbackToTypeApprover(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	approver2ID := createTestUser(t, router, adminToken, "approver2", "审批二", model.RoleApprover)

	// 配置类型默认审批人
	resp := request(t, router, http.MethodPut, "/api/v1/ticket-type-approvers/"+string(model.TicketTypeMaintenance), adminToken, map[string]interface{}{
		"approverId": approver2ID,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("set type approver status=%d body=%s", resp.Code, resp.Body.String())
	}

	// 资产 owner 不匹配任何系统用户
	assetID := createTestAsset(t, router, adminToken, "10.3.0.2", "dispatch-b", "不存在的负责人")

	configureEmptyWorkflow(t, router, adminToken, string(model.TicketTypeMaintenance), []string{"审批"})

	ticketID := createTestTicket(t, router, adminToken, string(model.TicketTypeMaintenance), "回退默认审批人", []uint{assetID})
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}

	approverIDs := firstStepApproverIDs(t, router, adminToken, ticketID)
	if !containsUint(approverIDs, approver2ID) {
		t.Fatalf("expected fallback type approver %d, got %v", approver2ID, approverIDs)
	}
}

// TestAutoDispatchKeepsConfiguredApprovers 验证节点已配置审批人时，不被资产负责人覆盖或追加。
func TestAutoDispatchKeepsConfiguredApprovers(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")

	// 资产负责人用户 + 流程节点配置的审批人
	createTestUser(t, router, adminToken, "zhangsan", "张三", model.RoleApprover)
	configuredID := createTestUser(t, router, adminToken, "configurer", "配置人", model.RoleApprover)

	assetID := createTestAsset(t, router, adminToken, "10.3.0.3", "dispatch-c", "zhangsan")

	// 配置流程：审批节点带审批人 configuredID
	resp := request(t, router, http.MethodPut, "/api/v1/workflows/"+string(model.TicketTypeMaintenance), adminToken, map[string]interface{}{
		"name":    "maintenance 流程",
		"enabled": true,
		"nodes":   []map[string]interface{}{{"name": "审批", "approverIds": []uint{configuredID}}},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("configure workflow status=%d body=%s", resp.Code, resp.Body.String())
	}

	ticketID := createTestTicket(t, router, adminToken, string(model.TicketTypeMaintenance), "保持配置审批人", []uint{assetID})
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticketID)+"/submit", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("submit status=%d body=%s", resp.Code, resp.Body.String())
	}

	approverIDs := firstStepApproverIDs(t, router, adminToken, ticketID)
	if len(approverIDs) != 1 || approverIDs[0] != configuredID {
		t.Fatalf("expected only configured approver %v, got %v", []uint{configuredID}, approverIDs)
	}
}

func createTestUser(t *testing.T, router http.Handler, token, username, name string, role model.Role) uint {
	t.Helper()
	resp := request(t, router, http.MethodPost, "/api/v1/users", token, map[string]interface{}{
		"username": username,
		"name":     name,
		"password": "user123456",
		"role":     role,
		"status":   "active",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", resp.Code, resp.Body.String())
	}
	var user model.User
	if err := json.Unmarshal(resp.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	return user.ID
}

func configureEmptyWorkflow(t *testing.T, router http.Handler, token, ticketType string, nodeNames []string) {
	t.Helper()
	nodes := make([]map[string]interface{}, 0, len(nodeNames))
	for _, name := range nodeNames {
		nodes = append(nodes, map[string]interface{}{"name": name, "approverIds": []uint{}})
	}
	resp := request(t, router, http.MethodPut, "/api/v1/workflows/"+ticketType, token, map[string]interface{}{
		"name":    ticketType + " 流程",
		"enabled": true,
		"nodes":   nodes,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("configure workflow status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func createTestTicket(t *testing.T, router http.Handler, token, ticketType, title string, assetIDs []uint) uint {
	t.Helper()
	body := map[string]interface{}{"type": ticketType, "title": title, "priority": "normal"}
	if len(assetIDs) > 0 {
		body["assetIds"] = assetIDs
	}
	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	return ticket.ID
}

func firstStepApproverIDs(t *testing.T, router http.Handler, token string, ticketID uint) []uint {
	t.Helper()
	resp := request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticketID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var detail struct {
		WorkflowSteps []struct {
			Approvers []struct {
				UserID uint `json:"userId"`
			} `json:"approvers"`
		} `json:"workflowSteps"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.WorkflowSteps) == 0 {
		t.Fatal("no workflow steps")
	}
	ids := make([]uint, 0, len(detail.WorkflowSteps[0].Approvers))
	for _, approver := range detail.WorkflowSteps[0].Approvers {
		ids = append(ids, approver.UserID)
	}
	return ids
}

func containsUint(list []uint, target uint) bool {
	for _, value := range list {
		if value == target {
			return true
		}
	}
	return false
}

package tests

import (
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

func TestTicketFlow(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	configureWorkflow(t, router, token, "asset_register", []string{"IT运维主管审核", "信息技术部经理审批"}, 1)

	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":        "asset_register",
		"title":       "登记应用服务器",
		"priority":    "normal",
		"description": "新服务器上线",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"submit", "approve", "approve", "start", "complete", "accept"} {
		payload := map[string]string{"remark": action, "result": action, "acceptanceResult": "验收通过"}
		resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/"+action, token, payload)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", action, resp.Code, resp.Body.String())
		}
	}
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticket.ID)+"/archive/download", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("download archive status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/archives/download", token, map[string]interface{}{"ids": []uint{ticket.ID}})
	if resp.Code != http.StatusOK {
		t.Fatalf("batch download archive status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("expected zip content-type, got %s", got)
	}
}

func TestTicketArchiveBatchRejectsInvalidItems(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	configureWorkflow(t, router, token, "asset_register", []string{"IT运维主管审核"}, 1)

	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":  "asset_register",
		"title": "未关闭工单",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/archives/download", token, map[string]interface{}{"ids": []uint{ticket.ID}})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected unclosed ticket to fail, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTicketArchiveDownloadRejectsNonParticipant(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")
	resp := request(t, router, http.MethodPost, "/api/v1/users", adminToken, map[string]interface{}{
		"username": "outsider",
		"name":     "无关用户",
		"role":     "applicant",
		"status":   "active",
		"password": "password123",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create outsider status=%d body=%s", resp.Code, resp.Body.String())
	}
	outsiderToken := login(t, router, "outsider", "password123")
	configureWorkflow(t, router, adminToken, "asset_register", []string{"IT运维主管审核"}, 1)
	resp = request(t, router, http.MethodPost, "/api/v1/tickets", adminToken, map[string]interface{}{
		"type":  "asset_register",
		"title": "已归档工单",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"submit", "approve", "start", "complete", "accept"} {
		resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/"+action, adminToken, map[string]string{"remark": action, "result": action, "acceptanceResult": "验收通过"})
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", action, resp.Code, resp.Body.String())
		}
	}
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticket.ID)+"/archive/download", outsiderToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected single archive forbidden, status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/archives/download", outsiderToken, map[string]interface{}{"ids": []uint{ticket.ID}})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected batch archive forbidden, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestSubmitTicketRequiresWorkflow(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":  "asset_change",
		"title": "变更服务器配置",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}

	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/submit", token, map[string]string{"remark": "submit"})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected missing workflow config to fail, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTicketTodoCommentsAndAttachments(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	configureWorkflow(t, router, token, "maintenance", []string{"IT运维主管审核"}, 1)

	resp := request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":  "maintenance",
		"title": "维护窗口申请",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ticket model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/submit", token, map[string]string{"remark": "submit"})
	if resp.Code != http.StatusOK {
		t.Fatalf("submit ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/tickets?view=todo", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("todo tickets status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/comments", token, map[string]string{"content": "请审批"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create comment status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = multipartRequest(t, router, "/api/v1/tickets/"+itoa(ticket.ID)+"/attachments", token, "file", "note.txt", "hello")
	if resp.Code != http.StatusCreated {
		t.Fatalf("upload attachment status=%d body=%s", resp.Code, resp.Body.String())
	}
}

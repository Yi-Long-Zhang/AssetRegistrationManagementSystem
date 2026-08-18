package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

func TestMailConfigAndSubmitNotification(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodPost, "/api/v1/users", token, map[string]interface{}{
		"username": "approver1",
		"name":     "审批人一",
		"email":    "approver1@example.com",
		"role":     "approver",
		"status":   "active",
		"password": "password123",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create approver status=%d body=%s", resp.Code, resp.Body.String())
	}
	var approver model.User
	if err := json.Unmarshal(resp.Body.Bytes(), &approver); err != nil {
		t.Fatal(err)
	}

	resp = request(t, router, http.MethodPut, "/api/v1/settings/mail", token, map[string]interface{}{
		"enabled":     true,
		"smtpHost":    "smtp.example.com",
		"smtpPort":    587,
		"username":    "smtp-user",
		"password":    "smtp-secret",
		"fromAddress": "asset-system@example.com",
		"fromName":    "资产管理系统",
		"startTls":    true,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("save mail config status=%d body=%s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("smtp-secret")) {
		t.Fatal("response leaked smtp password")
	}
	resp = request(t, router, http.MethodPost, "/api/v1/settings/mail/test", token, map[string]string{"recipient": "admin@example.com"})
	if resp.Code != http.StatusOK {
		t.Fatalf("test mail status=%d body=%s", resp.Code, resp.Body.String())
	}

	configureWorkflow(t, router, token, "maintenance", []string{"IT运维主管审核"}, approver.ID)
	resp = request(t, router, http.MethodPost, "/api/v1/tickets", token, map[string]interface{}{
		"type":        "maintenance",
		"title":       "维护窗口申请",
		"description": "需要审批通知",
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
	resp = request(t, router, http.MethodGet, "/api/v1/tickets/"+itoa(ticket.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("detail ticket status=%d body=%s", resp.Code, resp.Body.String())
	}
	var detail model.Ticket
	if err := json.Unmarshal(resp.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range detail.Records {
		if record.Action == "mail_sent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mail_sent record, got %+v", detail.Records)
	}
}

func TestADConfigImportAndLogin(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodPut, "/api/v1/ad/config", token, map[string]interface{}{
		"enabled":          true,
		"ldapUrl":          "ldap://ad.example.com:389",
		"baseDn":           "dc=example,dc=com",
		"bindDn":           "cn=svc,dc=example,dc=com",
		"bindPassword":     "bind-secret",
		"loginAttribute":   "sAMAccountName",
		"filterUserObject": true,
		"excludeDisabled":  true,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("save ad config status=%d body=%s", resp.Code, resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("bind-secret")) {
		t.Fatal("response leaked bind password")
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("objectClass=user")) {
		t.Fatal("response did not include generated user object filter")
	}

	resp = request(t, router, http.MethodPost, "/api/v1/ad/test", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("test ad status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/ad/lookup-user", token, map[string]string{"username": "zhangsan"})
	if resp.Code != http.StatusOK {
		t.Fatalf("lookup ad user status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/ad/import-user", token, map[string]string{"username": "zhangsan", "role": "approver", "status": "active"})
	if resp.Code != http.StatusOK {
		t.Fatalf("import ad user status=%d body=%s", resp.Code, resp.Body.String())
	}
	_ = login(t, router, "zhangsan", "ad-password")
}

func TestUnimportedADUserCannotLogin(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	resp := request(t, router, http.MethodPut, "/api/v1/ad/config", token, map[string]interface{}{
		"enabled":      true,
		"ldapUrl":      "ldap://ad.example.com:389",
		"baseDn":       "dc=example,dc=com",
		"bindDn":       "cn=svc,dc=example,dc=com",
		"bindPassword": "bind-secret",
		"userFilter":   "(sAMAccountName=%s)",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("save ad config status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "zhangsan", "password": "ad-password"})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unimported AD user login to fail, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestADUserPasswordCannotBeChangedLocally(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	request(t, router, http.MethodPut, "/api/v1/ad/config", token, map[string]interface{}{
		"enabled": true, "ldapUrl": "ldap://ad.example.com:389", "baseDn": "dc=example,dc=com", "bindDn": "cn=svc,dc=example,dc=com", "bindPassword": "bind-secret",
	})
	resp := request(t, router, http.MethodPost, "/api/v1/ad/import-user", token, map[string]string{"username": "zhangsan", "role": "applicant", "status": "active"})
	if resp.Code != http.StatusOK {
		t.Fatalf("import ad user status=%d body=%s", resp.Code, resp.Body.String())
	}
	var user model.User
	if err := json.Unmarshal(resp.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	resp = request(t, router, http.MethodPut, "/api/v1/users/"+itoa(user.ID), token, map[string]interface{}{
		"username": user.Username, "name": user.Name, "role": user.Role, "status": user.Status, "password": "new-local-password",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected AD password change to fail, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

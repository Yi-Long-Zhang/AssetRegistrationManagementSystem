package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
)

func TestLoginAndAssetCRUD(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	body := map[string]interface{}{
		"assetNo":  "SRV-001",
		"hostname": "srv-app-01",
		"ip":       "10.0.0.11",
		"status":   "in_use",
		"owner":    "平台组",
	}
	resp := request(t, router, http.MethodPost, "/api/v1/assets", token, body)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create asset status=%d body=%s", resp.Code, resp.Body.String())
	}

	resp = request(t, router, http.MethodGet, "/api/v1/assets", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list assets status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestSwaggerRouteRequiresExplicitEnable(t *testing.T) {
	router := testRouter(t)
	resp := request(t, router, http.MethodGet, "/swagger/index.html", "", nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected swagger disabled by default, status=%d body=%s", resp.Code, resp.Body.String())
	}

	router = testRouterWithConfig(t, config.Config{Swagger: config.SwaggerConfig{Enabled: true}})
	resp = request(t, router, http.MethodGet, "/swagger/index.html", "", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected swagger enabled, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestAssetImportExportAndTemplate(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	resp := request(t, router, http.MethodGet, "/api/v1/assets/template", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("download template status=%d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected xlsx template content-type, got %s", got)
	}
	templateRows, err := readXLSXRows(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
	if err != nil {
		t.Fatalf("read xlsx template: %v", err)
	}
	if len(templateRows) == 0 || len(templateRows[0]) < 3 || templateRows[0][0] != "序号" || templateRows[0][1] != "IP地址" || templateRows[0][2] != "主机名/设备名称" {
		t.Fatalf("template missing survey headers: %#v", templateRows)
	}
	if zipContains(resp.Body.Bytes(), "风险等级") {
		t.Fatal("xlsx template must not include risk level")
	}

	resp = request(t, router, http.MethodGet, "/api/v1/assets/template?format=csv", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("download csv template status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("IP地址")) || !bytes.Contains(resp.Body.Bytes(), []byte("运行服务/应用")) {
		t.Fatal("csv template missing survey headers")
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("风险等级")) {
		t.Fatal("csv template must not include risk level")
	}

	csvContent := "\ufeff序号,IP地址,主机名/设备名称,MAC地址,厂商,资产类型,操作系统,开放端口,运行服务/应用,应用版本,资产归属/负责人,所在网段,风险等级,备注\n17,10.0.0.21,17,17,17,服务器,17,22,17,17,17,办公网,高,17\n"
	resp = multipartRequest(t, router, "/api/v1/assets/import", token, "file", "assets.csv", csvContent)
	if resp.Code != http.StatusOK {
		t.Fatalf("import asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	var imported struct {
		Created int `json:"created"`
		Updated int `json:"updated"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Created != 1 || imported.Updated != 0 {
		t.Fatalf("expected created=1 updated=0, got created=%d updated=%d", imported.Created, imported.Updated)
	}
	resp = request(t, router, http.MethodGet, "/api/v1/assets?q=10.0.0.21&page=1&pageSize=10&sortBy=sequenceNo&sortOrder=asc", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list imported asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	var listed struct {
		Items    []model.Asset `json:"items"`
		Total    int64         `json:"total"`
		Page     int           `json:"page"`
		PageSize int           `json:"pageSize"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 || listed.Page != 1 || listed.PageSize != 10 || len(listed.Items) != 1 {
		t.Fatalf("unexpected paged list: %+v", listed)
	}
	cleaned := listed.Items[0]
	if cleaned.SequenceNo != "17" {
		t.Fatalf("sequence number 17 should be preserved, got %q", cleaned.SequenceNo)
	}
	if cleaned.Hostname != "" || cleaned.MACAddress != "" || cleaned.Manufacturer != "" || cleaned.OS != "" || cleaned.RunningServices != "" || cleaned.AppVersion != "" || cleaned.Owner != "" || cleaned.Remark != "" {
		t.Fatalf("placeholder 17 fields should be cleaned: %+v", cleaned)
	}

	csvContent = "序号,IP地址,主机名/设备名称,MAC地址,厂商,资产类型,操作系统,开放端口,运行服务/应用,应用版本,资产归属/负责人,所在网段,备注\n1,10.0.0.21,srv-db-01,AA:BB,华为,服务器,Linux,22,OpenSSH,9.0,李四,办公网,已复核\n"
	resp = multipartRequest(t, router, "/api/v1/assets/import", token, "file", "assets.csv", csvContent)
	if resp.Code != http.StatusOK {
		t.Fatalf("update imported asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Created != 0 || imported.Updated != 1 {
		t.Fatalf("expected created=0 updated=1, got created=%d updated=%d", imported.Created, imported.Updated)
	}

	xlsxContent := buildTestXLSX([][]string{
		{"全资产详细清单", "全资产详细清单", "全资产详细清单", "全资产详细清单", "全资产详细清单", "全资产详细清单"},
		{"序号", "IP地址", "主机名/设备名称", "资产类型", "运行服务/应用", "资产归属/负责人"},
		{"2", "10.0.0.31", "srv-app-xlsx", "服务器", "OA", "王五"},
	})
	resp = multipartBytesRequest(t, router, "/api/v1/assets/import", token, "file", "assets.xlsx", xlsxContent)
	if resp.Code != http.StatusOK {
		t.Fatalf("import xlsx asset status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Created != 1 || imported.Updated != 0 {
		t.Fatalf("expected xlsx created=1 updated=0, got created=%d updated=%d", imported.Created, imported.Updated)
	}

	resp = request(t, router, http.MethodGet, "/api/v1/assets?assetType=服务器&subnet=办公网&owner=李四&openPort=22&service=OpenSSH&sortBy=ip&sortOrder=asc&page=1&pageSize=20", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("filtered assets status=%d body=%s", resp.Code, resp.Body.String())
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.Total != 1 || len(listed.Items) != 1 || listed.Items[0].IP != "10.0.0.21" {
		t.Fatalf("unexpected filtered assets: %+v", listed)
	}

	resp = request(t, router, http.MethodGet, "/api/v1/assets/stats?subnet=办公网", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("asset stats status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte(`"total":1`)) || bytes.Contains(resp.Body.Bytes(), []byte("风险等级")) {
		t.Fatalf("unexpected stats body=%s", resp.Body.String())
	}

	resp = request(t, router, http.MethodGet, "/api/v1/assets/export?q=OpenSSH&sortBy=ip&sortOrder=asc", token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("export assets status=%d body=%s", resp.Code, resp.Body.String())
	}
	if !bytes.Contains(resp.Body.Bytes(), []byte("10.0.0.21")) || !bytes.Contains(resp.Body.Bytes(), []byte("李四")) {
		t.Fatalf("export missing imported asset body=%s", resp.Body.String())
	}
	if bytes.Contains(resp.Body.Bytes(), []byte("风险等级")) {
		t.Fatal("export must not include risk level")
	}
}

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

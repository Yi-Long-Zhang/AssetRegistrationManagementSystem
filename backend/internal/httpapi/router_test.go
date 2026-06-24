package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
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
	configureApprover(t, router, token, "asset_register", 1)

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

	for _, action := range []string{"submit", "approve", "start", "complete", "close"} {
		resp = request(t, router, http.MethodPost, "/api/v1/tickets/"+itoa(ticket.ID)+"/"+action, token, map[string]string{"remark": action})
		if resp.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", action, resp.Code, resp.Body.String())
		}
	}
}

func TestSubmitTicketRequiresTypeApprover(t *testing.T) {
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
		t.Fatalf("expected missing approver config to fail, status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTicketTodoCommentsAndAttachments(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")
	configureApprover(t, router, token, "maintenance", 1)

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

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := database.SeedAdmin(db, "admin", "admin123456"); err != nil {
		t.Fatal(err)
	}
	return NewRouter(Dependencies{
		Config: config.Config{
			HTTPAddr:       ":0",
			DatabasePath:   "test.db",
			AttachmentDir:  dir + "/attachments",
			JWTSecret:      "test-secret",
			ConfigKey:      "test-config-key",
			AuthMode:       "mixed",
			TokenTTL:       time.Hour,
			AllowedOrigins: "*",
		},
		DB:    db,
		Roles: model.AllRoles(),
		AD:    fakeADClient{},
	})
}

type fakeADClient struct{}

func (fakeADClient) Test(_ model.ADConfig, bindPassword string) error {
	if bindPassword != "bind-secret" {
		return fmt.Errorf("invalid bind password")
	}
	return nil
}

func (fakeADClient) LookupUser(_ model.ADConfig, _ string, username string) (service.ADUserInfo, error) {
	if username != "zhangsan" {
		return service.ADUserInfo{}, fmt.Errorf("not found")
	}
	return service.ADUserInfo{
		Username:    "zhangsan",
		DN:          "cn=zhangsan,dc=example,dc=com",
		DisplayName: "张三",
		Email:       "zhangsan@example.com",
		Department:  "运维部",
	}, nil
}

func (fakeADClient) Authenticate(config model.ADConfig, bindPassword, username, password string) (service.ADUserInfo, error) {
	if password != "ad-password" {
		return service.ADUserInfo{}, fmt.Errorf("invalid credentials")
	}
	return fakeADClient{}.LookupUser(config, bindPassword, username)
}

func configureApprover(t *testing.T, router http.Handler, token, ticketType string, approverID uint) {
	t.Helper()
	resp := request(t, router, http.MethodPut, "/api/v1/ticket-type-approvers/"+ticketType, token, map[string]uint{"approverId": approverID})
	if resp.Code != http.StatusOK {
		t.Fatalf("configure approver status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func login(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	resp := request(t, router, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": username,
		"password": password,
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", resp.Code, resp.Body.String())
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Token == "" {
		t.Fatal("empty token")
	}
	return data.Token
}

func multipartRequest(t *testing.T, router http.Handler, path, token, field, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	return multipartBytesRequest(t, router, path, token, field, filename, []byte(content))
}

func multipartBytesRequest(t *testing.T, router http.Handler, path, token, field, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func buildTestXLSX(rows [][]string) []byte {
	var body bytes.Buffer
	zipWriter := zip.NewWriter(&body)
	sheet, _ := zipWriter.Create("xl/worksheets/sheet1.xml")
	_, _ = sheet.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`))
	for rowIndex, row := range rows {
		_, _ = sheet.Write([]byte(fmt.Sprintf(`<row r="%d">`, rowIndex+1)))
		for colIndex, value := range row {
			cellRef := fmt.Sprintf("%s%d", excelColumnName(colIndex), rowIndex+1)
			_, _ = sheet.Write([]byte(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, cellRef, value)))
		}
		_, _ = sheet.Write([]byte(`</row>`))
	}
	_, _ = sheet.Write([]byte(`</sheetData></worksheet>`))
	_ = zipWriter.Close()
	return body.Bytes()
}

func zipContains(content []byte, term string) bool {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err == nil && bytes.Contains(data, []byte(term)) {
			return true
		}
	}
	return false
}

func excelColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
}

func request(t *testing.T, router http.Handler, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func itoa(id uint) string {
	return json.Number(fmtUint(id)).String()
}

func fmtUint(id uint) string {
	return strconvFormatUint(uint64(id))
}

func strconvFormatUint(id uint64) string {
	const digits = "0123456789"
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = digits[id%10]
		id /= 10
	}
	return string(buf[i:])
}

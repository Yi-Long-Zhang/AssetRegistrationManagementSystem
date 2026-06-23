package httpapi

import (
	"bytes"
	"encoding/json"
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

func (fakeADClient) Test(config model.ADConfig, bindPassword string) error {
	if bindPassword != "bind-secret" {
		return fmt.Errorf("invalid bind password")
	}
	return nil
}

func (fakeADClient) LookupUser(config model.ADConfig, bindPassword, username string) (service.ADUserInfo, error) {
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
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
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

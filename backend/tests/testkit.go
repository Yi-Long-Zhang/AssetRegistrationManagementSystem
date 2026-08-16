package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
	"asset-registration-management-system/backend/internal/httpapi"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

func testRouter(t *testing.T) http.Handler {
	return testRouterWithConfig(t, config.Config{})
}

func testRouterWithConfig(t *testing.T, cfgOverride config.Config) http.Handler {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
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

	cfg := config.Default()
	cfg.App.Env = config.EnvTest
	cfg.HTTP.Addr = ":0"
	cfg.Storage.DatabasePath = filepath.Join(dir, "test.db")
	cfg.Storage.AttachmentDir = filepath.Join(dir, "attachments")
	cfg.Storage.TicketArchiveDir = filepath.Join(dir, "archives")
	cfg.Storage.BackupDir = filepath.Join(dir, "backups")
	cfg.Security.JWTSecret = "test-secret"
	cfg.Security.ConfigEncryptionKey = "test-config-key"
	cfg.Auth.Mode = "mixed"
	cfg.CORS.AllowedOrigins = "*"
	mergeConfig(&cfg, cfgOverride)

	return httpapi.NewRouter(httpapi.Dependencies{
		Config:   cfg,
		DB:       db,
		Roles:    model.AllRoles(),
		AD:       fakeADClient{},
		Archiver: fakeArchiver{},
		Mail:     fakeMailSender{},
	})
}

func mergeConfig(target *config.Config, override config.Config) {
	if override.App.Env != "" {
		target.App.Env = override.App.Env
	}
	if override.HTTP.Addr != "" {
		target.HTTP.Addr = override.HTTP.Addr
	}
	if override.Storage.DatabasePath != "" {
		target.Storage.DatabasePath = override.Storage.DatabasePath
	}
	if override.Storage.AttachmentDir != "" {
		target.Storage.AttachmentDir = override.Storage.AttachmentDir
	}
	if override.Storage.TicketArchiveDir != "" {
		target.Storage.TicketArchiveDir = override.Storage.TicketArchiveDir
	}
	if override.Storage.TicketTemplatePath != "" {
		target.Storage.TicketTemplatePath = override.Storage.TicketTemplatePath
	}
	if override.Storage.LibreOfficeBin != "" {
		target.Storage.LibreOfficeBin = override.Storage.LibreOfficeBin
	}
	if override.Security.JWTSecret != "" {
		target.Security.JWTSecret = override.Security.JWTSecret
	}
	if override.Security.ConfigEncryptionKey != "" {
		target.Security.ConfigEncryptionKey = override.Security.ConfigEncryptionKey
	}
	if override.Auth.Mode != "" {
		target.Auth.Mode = override.Auth.Mode
	}
	if override.TokenTTL != 0 {
		target.TokenTTL = override.TokenTTL
	}
	if override.Admin.Username != "" {
		target.Admin.Username = override.Admin.Username
	}
	if override.Admin.Password != "" {
		target.Admin.Password = override.Admin.Password
	}
	if override.CORS.AllowedOrigins != "" {
		target.CORS.AllowedOrigins = override.CORS.AllowedOrigins
	}
	target.Swagger.Enabled = override.Swagger.Enabled
}

type fakeArchiver struct{}

func (fakeArchiver) Generate(_ context.Context, data service.TicketArchiveData, _ string, archiveDir string, _ string) (string, string, error) {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return "", "", err
	}
	archiveNo := fmt.Sprintf("ITCFG-TEST-%06d", data.Ticket.ID)
	archivePath := filepath.Join(archiveDir, archiveNo+".pdf")
	if err := os.WriteFile(archivePath, []byte("%PDF-1.4\n% test archive\n"), 0o644); err != nil {
		return "", "", err
	}
	return archiveNo, archivePath, nil
}

type fakeMailSender struct{}

func (fakeMailSender) Send(config model.MailConfig, password string, message service.MailMessage) error {
	if config.SMTPHost != "smtp.example.com" {
		return fmt.Errorf("invalid smtp host")
	}
	if password != "smtp-secret" {
		return fmt.Errorf("invalid smtp password")
	}
	if len(message.To) == 0 {
		return fmt.Errorf("empty recipients")
	}
	return nil
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

func (fakeADClient) Authenticate(cfg model.ADConfig, bindPassword, username, password string) (service.ADUserInfo, error) {
	if password != "ad-password" {
		return service.ADUserInfo{}, fmt.Errorf("invalid credentials")
	}
	return fakeADClient{}.LookupUser(cfg, bindPassword, username)
}

func configureWorkflow(t *testing.T, router http.Handler, token, ticketType string, nodeNames []string, approverID uint) {
	t.Helper()
	nodes := make([]map[string]interface{}, 0, len(nodeNames))
	for _, name := range nodeNames {
		nodes = append(nodes, map[string]interface{}{"name": name, "approverIds": []uint{approverID}})
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

func readXLSXRows(reader io.ReaderAt, size int64) ([][]string, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}
	sharedStrings, err := readSharedStrings(zipReader)
	if err != nil {
		return nil, err
	}
	for _, file := range zipReader.File {
		if file.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return parseSheetRows(rc, sharedStrings)
	}
	return nil, fmt.Errorf("sheet1.xml not found")
}

func readSharedStrings(zipReader *zip.Reader) ([]string, error) {
	for _, file := range zipReader.File {
		if file.Name != "xl/sharedStrings.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		var sst struct {
			Items []struct {
				Text string `xml:"t"`
			} `xml:"si"`
		}
		if err := xml.NewDecoder(rc).Decode(&sst); err != nil {
			return nil, err
		}
		values := make([]string, 0, len(sst.Items))
		for _, item := range sst.Items {
			values = append(values, item.Text)
		}
		return values, nil
	}
	return nil, nil
}

func parseSheetRows(reader io.Reader, sharedStrings []string) ([][]string, error) {
	decoder := xml.NewDecoder(reader)
	rows := [][]string{}
	var current []string
	inCell := false
	cellType := ""
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "row":
				current = []string{}
			case "c":
				inCell = true
				cellType = attrValue(element.Attr, "t")
			case "v", "t":
				if inCell {
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						return nil, err
					}
					if cellType == "s" {
						index := 0
						_, _ = fmt.Sscanf(value, "%d", &index)
						if index >= 0 && index < len(sharedStrings) {
							value = sharedStrings[index]
						}
					}
					current = append(current, value)
				}
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "c":
				inCell = false
				cellType = ""
			case "row":
				rows = append(rows, current)
			}
		}
	}
	return rows, nil
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func excelColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+index%26)) + name
		index = index/26 - 1
	}
	return name
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

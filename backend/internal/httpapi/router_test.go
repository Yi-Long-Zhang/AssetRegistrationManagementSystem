package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/database"
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

func TestTicketFlow(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

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

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(t.TempDir() + "/test.db")
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
			JWTSecret:      "test-secret",
			TokenTTL:       time.Hour,
			AllowedOrigins: "*",
		},
		DB:    db,
		Roles: model.AllRoles(),
	})
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

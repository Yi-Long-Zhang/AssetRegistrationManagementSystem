package tests

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"asset-registration-management-system/backend/internal/model"
)

func TestOperationalEndpointsAndRequestID(t *testing.T) {
	router := testRouter(t)
	resp := request(t, router, http.MethodGet, "/livez", "", nil)
	if resp.Code != http.StatusOK || resp.Header().Get("X-Request-ID") == "" {
		t.Fatalf("livez status=%d requestID=%q", resp.Code, resp.Header().Get("X-Request-ID"))
	}

	resp = request(t, router, http.MethodGet, "/readyz", "", nil)
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("test environment should report missing external dependencies, status=%d body=%s", resp.Code, resp.Body.String())
	}
	var ready struct {
		Status string `json:"status"`
		Checks map[string]struct {
			OK bool `json:"ok"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &ready); err != nil {
		t.Fatal(err)
	}
	if ready.Checks["database"].OK != true || ready.Checks["schedulers"].OK != false {
		t.Fatalf("unexpected readiness checks: %+v", ready.Checks)
	}

	resp = request(t, router, http.MethodGet, "/metrics", "", nil)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "arms_http_requests_total") ||
		!strings.Contains(resp.Body.String(), "arms_auth_sessions_active") {
		t.Fatalf("metrics status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestTaskCenterAuthorization(t *testing.T) {
	router := testRouter(t)
	adminToken := login(t, router, "admin", "admin123456")
	resp := request(t, router, http.MethodGet, "/api/v1/tasks", adminToken, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("admin task list status=%d body=%s", resp.Code, resp.Body.String())
	}
	createTestUser(t, router, adminToken, "task-viewer", "任务查看", model.RoleApplicant)
	viewerToken := login(t, router, "task-viewer", "user123456")
	resp = request(t, router, http.MethodGet, "/api/v1/tasks", viewerToken, nil)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("applicant task list status=%d body=%s", resp.Code, resp.Body.String())
	}
}

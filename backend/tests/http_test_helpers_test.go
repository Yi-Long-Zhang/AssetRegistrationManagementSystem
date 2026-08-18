package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

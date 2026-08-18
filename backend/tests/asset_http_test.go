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

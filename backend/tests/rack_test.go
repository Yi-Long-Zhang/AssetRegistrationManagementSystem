package tests

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRoomRackCRUD 验证机房与机柜 CRUD 及删除级联。
func TestRoomRackCRUD(t *testing.T) {
	router := testRouter(t)
	token := login(t, router, "admin", "admin123456")

	// 创建机房
	resp := request(t, router, http.MethodPost, "/api/v1/rooms", token, map[string]interface{}{
		"name": "A栋-3F机房", "location": "A栋3楼",
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create room status=%d body=%s", resp.Code, resp.Body.String())
	}
	var room struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &room); err != nil {
		t.Fatal(err)
	}

	// 创建机柜
	resp = request(t, router, http.MethodPost, "/api/v1/racks", token, map[string]interface{}{
		"roomId": room.ID, "name": "A-01", "units": 42,
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("create rack status=%d body=%s", resp.Code, resp.Body.String())
	}
	var rack struct {
		ID    uint `json:"id"`
		Units int  `json:"units"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &rack); err != nil {
		t.Fatal(err)
	}
	if rack.Units != 42 {
		t.Fatalf("expect units 42, got %d", rack.Units)
	}

	// 机柜列表（按机房过滤）
	resp = request(t, router, http.MethodGet, "/api/v1/racks?roomId="+itoa(room.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list racks status=%d body=%s", resp.Code, resp.Body.String())
	}
	var rackList struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &rackList); err != nil {
		t.Fatal(err)
	}
	if len(rackList.Items) != 1 {
		t.Fatalf("expect 1 rack, got %d", len(rackList.Items))
	}

	// 机房列表含机柜
	resp = request(t, router, http.MethodGet, "/api/v1/rooms", token, nil)
	var roomList struct {
		Items []struct {
			ID    uint                     `json:"id"`
			Racks []map[string]interface{} `json:"racks"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &roomList); err != nil {
		t.Fatal(err)
	}
	if len(roomList.Items) != 1 || len(roomList.Items[0].Racks) != 1 {
		t.Fatalf("expect room with 1 rack, got %+v", roomList.Items)
	}

	// 删除机房（级联删机柜）
	resp = request(t, router, http.MethodDelete, "/api/v1/rooms/"+itoa(room.ID), token, nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("delete room status=%d body=%s", resp.Code, resp.Body.String())
	}
	resp = request(t, router, http.MethodGet, "/api/v1/racks?roomId="+itoa(room.ID), token, nil)
	if err := json.Unmarshal(resp.Body.Bytes(), &rackList); err != nil {
		t.Fatal(err)
	}
	if len(rackList.Items) != 0 {
		t.Fatalf("expect 0 racks after room delete, got %d", len(rackList.Items))
	}
}

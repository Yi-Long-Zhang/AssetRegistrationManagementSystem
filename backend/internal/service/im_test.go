package service

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	cases := []struct {
		webhook string
		want    IMPlatform
	}{
		{"https://oapi.dingtalk.com/robot/send?access_token=x", IMPlatformDingTalk},
		{"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x", IMPlatformWeCom},
		{"https://open.feishu.cn/open-apis/bot/v2/hook/x", IMPlatformFeishu},
		{"https://example.com/hook", ""},
	}
	for _, c := range cases {
		if got := detectPlatform(c.webhook); got != c.want {
			t.Fatalf("detectPlatform(%s) = %s, want %s", c.webhook, got, c.want)
		}
	}
}

func TestDingTalkPayload(t *testing.T) {
	data, err := dingTalkPayload("标题", "内容", "")
	if err != nil {
		t.Fatalf("dingTalkPayload: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v, want markdown", m["msgtype"])
	}
	md := m["markdown"].(map[string]interface{})
	if md["title"] != "标题" || !strings.Contains(md["text"].(string), "内容") {
		t.Fatalf("payload mismatch: %s", data)
	}
}

func TestWeComPayload(t *testing.T) {
	data, err := wecomPayload("t", "hello **world**", "")
	if err != nil {
		t.Fatalf("wecomPayload: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["msgtype"] != "markdown" {
		t.Fatalf("msgtype = %v", m["msgtype"])
	}
}

func TestFeishuPayload(t *testing.T) {
	data, err := feishuPayload("卡片标题", "卡片内容", "")
	if err != nil {
		t.Fatalf("feishuPayload: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["msg_type"] != "interactive" {
		t.Fatalf("msg_type = %v", m["msg_type"])
	}
	card := m["card"].(map[string]interface{})
	if card["header"] == nil || card["elements"] == nil {
		t.Fatalf("card should have header and elements: %s", data)
	}
}

func TestAppendDingTalkSign(t *testing.T) {
	webhook := "https://oapi.dingtalk.com/robot/send?access_token=abc"
	got := appendDingTalkSign(webhook, "SEC123")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse signed url: %v", err)
	}
	q := u.Query()
	if q.Get("timestamp") == "" {
		t.Fatal("signed url should contain timestamp")
	}
	if q.Get("sign") == "" {
		t.Fatal("signed url should contain sign")
	}
	if strings.Contains(got, "access_token=abc?timestamp") {
		t.Fatal("sign params should be appended with & when query exists")
	}
}

func TestBuildTicketMessage(t *testing.T) {
	text := BuildTicketMessage("工单审批通过", "- 工单：#1 测试\n- 审批人：admin")
	if !strings.Contains(text, "### 工单审批通过") || !strings.Contains(text, "#1") || !strings.Contains(text, "登录系统") {
		t.Fatalf("unexpected message: %s", text)
	}
}

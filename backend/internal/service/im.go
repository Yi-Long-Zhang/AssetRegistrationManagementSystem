package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// IMPlatform 即时通讯平台
type IMPlatform string

const (
	IMPlatformDingTalk IMPlatform = "dingtalk"
	IMPlatformWeCom    IMPlatform = "wecom"
	IMPlatformFeishu   IMPlatform = "feishu"
)

// IMNotifier 群机器人消息发送器。
type IMNotifier interface {
	// SendText 发送文本/markdown 消息到群，返回错误。
	SendText(webhook, secret, title, text string) error
	// SendCard 发送带"查看详情"跳转按钮的卡片消息（群机器人仅支持跳转链接，不支持按钮回调）。
	SendCard(webhook, secret, title, text, linkURL string) error
}

// HTTPDoer 便于测试注入的 HTTP 客户端。
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type imNotifier struct {
	client HTTPDoer
}

// NewIMNotifier 创建群机器人通知器。
func NewIMNotifier() IMNotifier {
	return &imNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}

// SendText 按平台发送卡片消息；secret 非空时附加签名。
func (n *imNotifier) SendText(webhook, secret, title, text string) error {
	return n.send(webhook, secret, title, text, "")
}

// SendCard 发送带"查看详情"跳转按钮的卡片消息。
func (n *imNotifier) SendCard(webhook, secret, title, text, linkURL string) error {
	return n.send(webhook, secret, title, text, linkURL)
}

func (n *imNotifier) send(webhook, secret, title, text, linkURL string) error {
	platform := detectPlatform(webhook)
	var payload []byte
	var err error
	switch platform {
	case IMPlatformDingTalk:
		payload, err = dingTalkPayload(title, text, linkURL)
	case IMPlatformWeCom:
		payload, err = wecomPayload(title, text, linkURL)
	case IMPlatformFeishu:
		payload, err = feishuPayload(title, text, linkURL)
	default:
		return fmt.Errorf("无法识别的 webhook 平台")
	}
	if err != nil {
		return err
	}
	if secret != "" && platform == IMPlatformDingTalk {
		webhook = appendDingTalkSign(webhook, secret)
	}
	req, err := http.NewRequest(http.MethodPost, webhook, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("IM webhook 响应异常: HTTP %d", resp.StatusCode)
	}
	return nil
}

// detectPlatform 根据 webhook 域名识别平台。
func detectPlatform(webhook string) IMPlatform {
	switch {
	case strings.Contains(webhook, "oapi.dingtalk.com"):
		return IMPlatformDingTalk
	case strings.Contains(webhook, "qyapi.weixin.qq.com"):
		return IMPlatformWeCom
	case strings.Contains(webhook, "open.feishu.cn"):
		return IMPlatformFeishu
	}
	return ""
}

// dingTalkPayload 钉钉 markdown 消息（可选 actionCard 跳转按钮）。
func dingTalkPayload(title, text, linkURL string) ([]byte, error) {
	if linkURL == "" {
		body := map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  text,
			},
		}
		return json.Marshal(body)
	}
	body := map[string]interface{}{
		"msgtype": "actionCard",
		"actionCard": map[string]interface{}{
			"title":          title,
			"text":           text,
			"btnOrientation": "1",
			"btns": []map[string]string{
				{"title": "查看详情", "actionURL": linkURL},
			},
		},
	}
	return json.Marshal(body)
}

// wecomPayload 企业微信 markdown 消息（附加跳转链接文本）。
func wecomPayload(title, text, linkURL string) ([]byte, error) {
	content := text
	if linkURL != "" {
		content += "\n\n[查看详情](" + linkURL + ")"
	}
	body := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
	return json.Marshal(body)
}

// feishuPayload 飞书 interactive 卡片（可选"查看详情"跳转按钮）。
func feishuPayload(title, text, linkURL string) ([]byte, error) {
	elements := []map[string]interface{}{
		{"tag": "markdown", "content": text},
	}
	if linkURL != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "action",
			"actions": []map[string]interface{}{
				{
					"tag":  "button",
					"text": map[string]string{"tag": "plain_text", "content": "查看详情"},
					"type": "primary",
					"url":  linkURL,
				},
			},
		})
	}
	body := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{"tag": "plain_text", "content": title},
			},
			"elements": elements,
		},
	}
	return json.Marshal(body)
}

// appendDingTalkSign 钉钉加签：timestamp + sign 追加到 webhook。
func appendDingTalkSign(webhook, secret string) string {
	timestamp := time.Now().UnixMilli()
	strToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strToSign))
	sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	sep := "?"
	if strings.Contains(webhook, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhook, sep, timestamp, sign)
}

// BuildTicketMessage 构造工单通知 markdown 文本。
func BuildTicketMessage(title, detail string) string {
	var b strings.Builder
	b.WriteString("### " + title + "\n")
	if detail != "" {
		b.WriteString(detail + "\n")
	}
	b.WriteString("---\n")
	b.WriteString("请登录系统查看工单详情。")
	return b.String()
}

// SendIMNotification 读取 IM 配置并发送群通知；未启用或失败时仅记日志返回 nil。
// secret 若加密存储则由 encryptionKey 解密；返回 (sent bool, err error)。
func SendIMNotification(db *gorm.DB, notifier IMNotifier, encryptionKey, title, text string) (bool, error) {
	var cfg model.IMConfig
	if err := db.First(&cfg).Error; err != nil {
		return false, nil // 未配置 IM，静默跳过
	}
	if !cfg.Enabled || strings.TrimSpace(cfg.Webhook) == "" {
		return false, nil
	}
	secret := cfg.Secret
	if cfg.EncryptedSecret && secret != "" && encryptionKey != "" {
		if dec, err := DecryptString(secret, encryptionKey); err == nil {
			secret = dec
		}
	}
	if notifier == nil {
		notifier = NewIMNotifier()
	}
	if err := notifier.SendText(cfg.Webhook, secret, title, text); err != nil {
		return false, err
	}
	return true, nil
}

// VerifyIMSignature 校验 IM 回调签名：sign = hex(hmac-sha256(secret, body))，
// timestamp 与当前时间差超过 windowSec 秒拒绝（防重放）。constant-time 比较防时序攻击。
func VerifyIMSignature(secret, sign, timestamp string, body []byte, windowSec int64) bool {
	if sign == "" {
		return false
	}
	if ts, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		if diff := time.Now().Unix() - ts; diff > windowSec || diff < -windowSec {
			return false
		}
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(sign)), []byte(expected)) == 1
}

// gormDBLike 兼容 gorm.DB 的最小接口（保留扩展用）。
type gormDBLike interface {
	First(dest interface{}, conds ...interface{}) (*gorm.DB, error)
}

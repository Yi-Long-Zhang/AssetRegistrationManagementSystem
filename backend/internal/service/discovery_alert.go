package service

import (
	"log"
	"net/mail"
	"strconv"
	"strings"

	"asset-registration-management-system/backend/internal/model"
)

// sendDiscoveryAlert 在发现运行完成后发送变更告警邮件（如有高风险变更/新增主机且配置了收件人）。
// 邮件配置从数据库读取（与工单邮件共用）；发送失败仅记日志，不影响主流程。
func (s *DiscoveryService) sendDiscoveryAlert(rule model.DiscoveryRule, run *model.DiscoveryRun) {
	emails := s.Config.Discovery.AlertEmails
	if strings.TrimSpace(emails) == "" {
		return
	}
	if run.NewCount == 0 && run.ChangedCount == 0 && run.OfflineCount == 0 {
		return // 无变更，不打扰
	}

	var mailConfig model.MailConfig
	if err := s.DB.First(&mailConfig).Error; err != nil {
		log.Printf("discovery alert: mail config not found: %v", err)
		return
	}
	if !mailConfig.Enabled || strings.TrimSpace(mailConfig.SMTPHost) == "" {
		return
	}
	password := ""
	if mailConfig.EncryptedPassword != "" {
		pw, err := DecryptString(mailConfig.EncryptedPassword, s.Config.Security.ConfigEncryptionKey)
		if err != nil {
			log.Printf("discovery alert: decrypt mail password: %v", err)
			return
		}
		password = pw
	}

	var to []mail.Address
	for _, addr := range strings.Split(emails, ",") {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		to = append(to, mail.Address{Address: addr})
	}
	if len(to) == 0 {
		return
	}

	sender := s.MailSender
	if sender == nil {
		sender = SMTPMailSender{}
	}
	subject := "[资产发现] " + rule.Name + " 发现变更告警"
	body := buildAlertBody(rule, run)
	msg := MailMessage{To: to, Subject: subject, Body: body}
	if err := sender.Send(mailConfig, password, msg); err != nil {
		log.Printf("discovery alert: send mail: %v", err)
	}
}

// buildAlertBody 构造告警邮件 HTML 正文。
func buildAlertBody(rule model.DiscoveryRule, run *model.DiscoveryRun) string {
	var b strings.Builder
	b.WriteString("<h3>资产自动发现变更告警</h3>")
	b.WriteString("<p>规则：<b>" + rule.Name + "</b>（#" + strconv.Itoa(int(run.RuleID)) + "）</p>")
	b.WriteString("<p>运行时间：" + run.StartedAt.Format("2006-01-02 15:04:05") + "</p>")
	b.WriteString("<table border='1' cellpadding='6' cellspacing='0' style='border-collapse:collapse'>")
	b.WriteString("<tr><th>新增</th><th>变更</th><th>离线</th><th>恢复在线</th></tr>")
	b.WriteString("<tr><td>" + strconv.Itoa(run.NewCount) + "</td><td>" + strconv.Itoa(run.ChangedCount) + "</td><td>" +
		strconv.Itoa(run.OfflineCount) + "</td><td>" + strconv.Itoa(run.OnlineCount) + "</td></tr>")
	b.WriteString("</table>")
	b.WriteString("<p>请登录系统查看运行详情并处理高风险变更（端口关闭/OS 变化等需人工确认）。</p>")
	return b.String()
}

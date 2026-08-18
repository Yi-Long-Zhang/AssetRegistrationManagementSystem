package httpapi

import (
	"fmt"
	"net/mail"
	"strings"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

func (h *Handler) addRecord(ticketID, actorID uint, action string, from, to model.TicketStatus, remark string) {
	_ = h.db.Create(&model.TicketRecord{
		TicketID:   ticketID,
		ActorID:    actorID,
		Action:     action,
		FromStatus: from,
		ToStatus:   to,
		Remark:     remark,
	}).Error
}

func (h *Handler) notifyCurrentApprovers(ticketID uint) {
	mailConfig := h.currentMailConfig()
	if mailConfig.ID == 0 || !mailConfig.Enabled {
		return
	}
	password, err := h.mailConfigPassword(mailConfig)
	if err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "SMTP 密码解密失败")
		return
	}
	var ticket model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").First(&ticket, ticketID).Error; err != nil {
		return
	}
	if ticket.CurrentWorkflowStepID == nil {
		return
	}
	var step model.TicketWorkflowStep
	if err := h.db.Preload("Approvers.User").First(&step, *ticket.CurrentWorkflowStepID).Error; err != nil {
		return
	}
	recipients := make([]mail.Address, 0, len(step.Approvers))
	for _, approver := range step.Approvers {
		address, err := mail.ParseAddress(strings.TrimSpace(approver.User.Email))
		if err != nil {
			continue
		}
		if approver.User.Name != "" {
			address.Name = approver.User.Name
		}
		recipients = append(recipients, *address)
	}
	if len(recipients) == 0 {
		h.addSystemRecord(ticketID, "mail_skipped", "当前审批节点没有可用审批人邮箱")
		return
	}
	message := service.MailMessage{
		To:      recipients,
		Subject: fmt.Sprintf("待审批工单 #%d：%s", ticket.ID, ticket.Title),
		Body:    approvalMailBody(ticket, step),
	}
	if err := h.mail.Send(mailConfig, password, message); err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "审批通知邮件发送失败: "+err.Error())
		return
	}
	h.addSystemRecord(ticketID, "mail_sent", fmt.Sprintf("已通知审批节点 %s，共 %d 人", step.Name, len(recipients)))
}

func (h *Handler) addSystemRecord(ticketID uint, action, remark string) {
	var ticket model.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		return
	}
	h.addRecord(ticketID, ticket.ApplicantID, action, ticket.Status, ticket.Status, remark)
}

func approvalMailBody(ticket model.Ticket, step model.TicketWorkflowStep) string {
	lines := []string{
		"您好，",
		"",
		"有一张工单等待您审批。",
		"",
		fmt.Sprintf("工单编号：#%d", ticket.ID),
		"工单标题：" + ticket.Title,
		"审批节点：" + step.Name,
		"工单类型：" + string(ticket.Type),
		"优先级：" + string(ticket.Priority),
		"申请人：" + defaultString(ticket.Applicant.Name, ticket.Applicant.Username),
	}
	if ticket.Asset != nil {
		lines = append(lines, "关联资产："+defaultString(ticket.Asset.Hostname, ticket.Asset.IP))
	}
	if strings.TrimSpace(ticket.Description) != "" {
		lines = append(lines, "", "申请说明：", ticket.Description)
	}
	lines = append(lines, "", "请登录资产管理系统处理。")
	return strings.Join(lines, "\n")
}

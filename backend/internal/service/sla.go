package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// SLAScheduler 进程内 SLA 超时扫描器：每分钟检查一次，
// 对已超时的审批/执行阶段工单标记并发送邮件 + IM 通知（每个工单仅通知一次）。
type SLAScheduler struct {
	db            *gorm.DB
	encryptionKey string
	stop          chan struct{}
	wg            sync.WaitGroup
	tasks         *TaskManager
}

// NewSLAScheduler 创建 SLA 扫描器。
func NewSLAScheduler(db *gorm.DB, encryptionKey string) *SLAScheduler {
	return &SLAScheduler{db: db, encryptionKey: encryptionKey}
}

func (s *SLAScheduler) WithTaskManager(tasks *TaskManager) *SLAScheduler {
	s.tasks = tasks
	return s
}

// Start 启动扫描循环（幂等：重复调用仅启动一次）。
func (s *SLAScheduler) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	if s.tasks != nil {
		s.tasks.Register("sla_overdue", s.runTask)
		s.tasks.ResumeKind(context.Background(), "sla_overdue")
	}
	s.wg.Add(1)
	go s.loop()
	log.Printf("sla scheduler started")
}

// Stop 停止扫描循环并等待进行中的任务结束。
func (s *SLAScheduler) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = nil
}

func (s *SLAScheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *SLAScheduler) tick() {
	now := time.Now()
	var tickets []model.Ticket
	err := s.db.Preload("Applicant").Preload("Approver").Preload("Executor").Where(
		"sla_overdue_notified = ? AND ("+
			"(status = ? AND sla_approval_deadline IS NOT NULL AND sla_approval_deadline < ?) OR "+
			"(status = ? AND sla_completion_deadline IS NOT NULL AND sla_completion_deadline < ?))",
		false,
		model.TicketStatusPendingApproval, now,
		model.TicketStatusInProgress, now,
	).Find(&tickets).Error
	if err != nil {
		log.Printf("sla scheduler: query overdue tickets: %v", err)
		return
	}
	for i := range tickets {
		t := &tickets[i]
		if s.tasks != nil {
			deadline := t.SLAApprovalDeadline
			if t.Status == model.TicketStatusInProgress {
				deadline = t.SLACompletionDeadline
			}
			deadlineUnix := int64(0)
			if deadline != nil {
				deadlineUnix = deadline.Unix()
			}
			if _, err := s.tasks.Run(context.Background(), "sla_overdue", "schedule",
				fmt.Sprintf("sla:%d:%d", t.ID, deadlineUnix),
				map[string]interface{}{"ticketId": t.ID}); err != nil {
				log.Printf("sla scheduler: task for ticket %d: %v", t.ID, err)
			}
			continue
		}
		t.SLAOverdueNotified = true
		if err := s.db.Save(t).Error; err != nil {
			log.Printf("sla scheduler: mark ticket %d overdue: %v", t.ID, err)
			continue
		}
		s.notify(t)
	}
}

func (s *SLAScheduler) runTask(_ context.Context, raw json.RawMessage) (interface{}, error) {
	var payload struct {
		TicketID uint `json:"ticketId"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var ticket model.Ticket
	if err := s.db.Preload("Applicant").Preload("Approver").Preload("Executor").First(&ticket, payload.TicketID).Error; err != nil {
		return nil, err
	}
	if ticket.SLAOverdueNotified {
		return map[string]interface{}{"skipped": true}, nil
	}
	ticket.SLAOverdueNotified = true
	if err := s.db.Save(&ticket).Error; err != nil {
		return nil, err
	}
	s.notify(&ticket)
	return map[string]interface{}{"ticketId": ticket.ID}, nil
}

// notify 为超时工单发送审计、IM 群通知与邮件提醒。
func (s *SLAScheduler) notify(t *model.Ticket) {
	_ = s.db.Create(&model.TicketRecord{
		TicketID:   t.ID,
		ActorID:    t.ApplicantID,
		Action:     "sla_overdue",
		FromStatus: t.Status,
		ToStatus:   t.Status,
		Remark:     "工单 SLA 已超时，系统自动提醒",
	}).Error

	text := BuildTicketMessage("工单 SLA 超时提醒",
		"- 工单：#"+strconv.FormatUint(uint64(t.ID), 10)+" "+t.Title+"\n- 当前状态："+slaStatusText(t.Status)+"\n- 请相关人员尽快处理。")
	if _, err := SendIMNotification(s.db, nil, s.encryptionKey, "工单 SLA 超时提醒", text); err != nil {
		log.Printf("sla scheduler: im notify ticket %d: %v", t.ID, err)
	}
	s.sendMail(t)
}

func (s *SLAScheduler) sendMail(t *model.Ticket) {
	var cfg model.MailConfig
	if err := s.db.First(&cfg).Error; err != nil || !cfg.Enabled {
		return // 未配置邮件，静默跳过
	}
	password := ""
	if cfg.EncryptedPassword != "" && s.encryptionKey != "" {
		if dec, err := DecryptString(cfg.EncryptedPassword, s.encryptionKey); err == nil {
			password = dec
		}
	}
	recipients := make([]mail.Address, 0, 3)
	// 申请人
	if addr, err := mail.ParseAddress(strings.TrimSpace(t.Applicant.Email)); err == nil && t.Applicant.Email != "" {
		if t.Applicant.Name != "" {
			addr.Name = t.Applicant.Name
		}
		recipients = append(recipients, *addr)
	}
	// 审批超时 → 当前审批人；执行超时 → 执行人
	if t.Status == model.TicketStatusPendingApproval && t.Approver != nil {
		if addr, err := mail.ParseAddress(strings.TrimSpace(t.Approver.Email)); err == nil && t.Approver.Email != "" {
			recipients = append(recipients, *addr)
		}
	}
	if t.Status == model.TicketStatusInProgress && t.Executor != nil {
		if addr, err := mail.ParseAddress(strings.TrimSpace(t.Executor.Email)); err == nil && t.Executor.Email != "" {
			recipients = append(recipients, *addr)
		}
	}
	if len(recipients) == 0 {
		return
	}
	message := MailMessage{
		To:      recipients,
		Subject: "工单 SLA 超时提醒 #" + strconv.FormatUint(uint64(t.ID), 10) + "：" + t.Title,
		Body:    "工单 #" + strconv.FormatUint(uint64(t.ID), 10) + "（" + t.Title + "）已超过 SLA 时限，当前状态：" + slaStatusText(t.Status) + "。请登录系统尽快处理。",
	}
	if err := (SMTPMailSender{}).Send(cfg, password, message); err != nil {
		log.Printf("sla scheduler: mail notify ticket %d: %v", t.ID, err)
	}
}

func slaStatusText(status model.TicketStatus) string {
	if status == model.TicketStatusPendingApproval {
		return "审批中"
	}
	if status == model.TicketStatusInProgress {
		return "执行中"
	}
	return string(status)
}

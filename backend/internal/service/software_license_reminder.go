package service

import (
	"context"
	"encoding/json"
	"log"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// LicenseRemindDays 许可证到期提醒窗口（默认未来 30 天内，含已过期）。
const LicenseRemindDays = 30

// LicenseReminderScheduler 许可证到期提醒扫描器：每天检查一次，
// 对许可证到期日在 [今天, 今天+30天] 或已过期的许可证发送邮件（admin）与 IM 群通知。
type LicenseReminderScheduler struct {
	db            *gorm.DB
	encryptionKey string
	lastDay       string
	stop          chan struct{}
	wg            sync.WaitGroup
	tasks         *TaskManager
}

// NewLicenseReminderScheduler 创建许可证提醒扫描器。
func NewLicenseReminderScheduler(db *gorm.DB, encryptionKey string) *LicenseReminderScheduler {
	return &LicenseReminderScheduler{db: db, encryptionKey: encryptionKey}
}

func (s *LicenseReminderScheduler) WithTaskManager(tasks *TaskManager) *LicenseReminderScheduler {
	s.tasks = tasks
	return s
}

// Start 启动扫描循环（幂等）。
func (s *LicenseReminderScheduler) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	if s.tasks != nil {
		s.tasks.Register("license_reminder", s.runTask)
		s.tasks.ResumeKind(context.Background(), "license_reminder")
	}
	s.wg.Add(1)
	go s.loop()
	log.Printf("license reminder scheduler started")
}

// Stop 停止扫描循环。
func (s *LicenseReminderScheduler) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = nil
}

func (s *LicenseReminderScheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(1 * time.Hour)
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

func (s *LicenseReminderScheduler) tick() {
	today := time.Now().Format("2006-01-02")
	if s.lastDay == today {
		return // 每天只提醒一次
	}
	s.lastDay = today
	if s.tasks != nil {
		if _, err := s.tasks.Run(context.Background(), "license_reminder", "schedule",
			"license:"+today, map[string]interface{}{"date": today}); err != nil {
			log.Printf("license reminder task: %v", err)
		}
		return
	}
	licenses := s.expiringLicenses(time.Now())
	if len(licenses) == 0 {
		return
	}
	s.notify(licenses)
	s.createRenewalTickets(licenses)
}

func (s *LicenseReminderScheduler) runTask(_ context.Context, _ json.RawMessage) (interface{}, error) {
	licenses := s.expiringLicenses(time.Now())
	if len(licenses) > 0 {
		s.notify(licenses)
		s.createRenewalTickets(licenses)
	}
	return map[string]interface{}{"count": len(licenses)}, nil
}

// expiringLicenses 查询许可证到期日在 [now, now+30天] 或已过期的许可证。
func (s *LicenseReminderScheduler) expiringLicenses(now time.Time) []model.SoftwareLicense {
	deadline := now.AddDate(0, 0, LicenseRemindDays)
	var licenses []model.SoftwareLicense
	if err := s.db.Where("expire_date IS NOT NULL AND expire_date <= ?", deadline).
		Order("expire_date asc").Find(&licenses).Error; err != nil {
		log.Printf("license reminder: query licenses: %v", err)
		return nil
	}
	return licenses
}

func (s *LicenseReminderScheduler) notify(licenses []model.SoftwareLicense) {
	// IM 群通知
	var b strings.Builder
	b.WriteString("- 即将到期或已过期许可证共 " + strconv.Itoa(len(licenses)) + " 个：\n")
	for _, l := range licenses {
		expire := ""
		if l.ExpireDate != nil {
			expire = l.ExpireDate.Format("2006-01-02")
		}
		b.WriteString("- " + l.Name + "（" + l.Vendor + "）到期 " + expire + "\n")
	}
	if _, err := SendIMNotification(s.db, nil, s.encryptionKey, "软件许可到期提醒", BuildTicketMessage("软件许可到期提醒", b.String())); err != nil {
		log.Printf("license reminder: im notify: %v", err)
	}
	s.sendMail(licenses)
}

func (s *LicenseReminderScheduler) sendMail(licenses []model.SoftwareLicense) {
	var cfg model.MailConfig
	if err := s.db.First(&cfg).Error; err != nil || !cfg.Enabled {
		return
	}
	password := ""
	if cfg.EncryptedPassword != "" && s.encryptionKey != "" {
		if dec, err := DecryptString(cfg.EncryptedPassword, s.encryptionKey); err == nil {
			password = dec
		}
	}
	// 收件人：所有 admin 用户
	var admins []model.User
	if err := s.db.Where("role = ? AND status = ?", model.RoleAdmin, "active").Find(&admins).Error; err != nil {
		return
	}
	recipients := make([]mail.Address, 0, len(admins))
	for _, u := range admins {
		if addr, err := mail.ParseAddress(strings.TrimSpace(u.Email)); err == nil && u.Email != "" {
			if u.Name != "" {
				addr.Name = u.Name
			}
			recipients = append(recipients, *addr)
		}
	}
	if len(recipients) == 0 {
		return
	}
	var b strings.Builder
	b.WriteString("以下软件许可证即将到期或已过期：\n\n")
	for _, l := range licenses {
		expire := ""
		if l.ExpireDate != nil {
			expire = l.ExpireDate.Format("2006-01-02")
		}
		b.WriteString("- " + l.Name + "（" + l.Vendor + "）到期 " + expire + "，授权 " + strconv.Itoa(l.TotalSeats) + " 个\n")
	}
	message := MailMessage{
		To:      recipients,
		Subject: "软件许可到期提醒（" + strconv.Itoa(len(licenses)) + " 个）",
		Body:    b.String(),
	}
	if err := (SMTPMailSender{}).Send(cfg, password, message); err != nil {
		log.Printf("license reminder: mail notify: %v", err)
	}
}

// createRenewalTickets 为到期许可证自动生成续费工单（草稿，需人工提交审批）。
// 申请人取首位 admin；同一许可证存在未关闭/未取消的续费工单时跳过，避免重复生成。
func (s *LicenseReminderScheduler) createRenewalTickets(licenses []model.SoftwareLicense) {
	var admin model.User
	if err := s.db.Where("role = ?", model.RoleAdmin).Order("id asc").First(&admin).Error; err != nil {
		log.Printf("license renewal ticket: no admin applicant: %v", err)
		return
	}
	openStatuses := []model.TicketStatus{model.TicketStatusClosed, model.TicketStatusCancelled}
	for i := range licenses {
		lic := licenses[i]
		var count int64
		if err := s.db.Model(&model.Ticket{}).
			Where("type = ? AND license_id = ? AND status NOT IN ?", model.TicketTypeLicenseRenew, lic.ID, openStatuses).
			Count(&count).Error; err != nil {
			log.Printf("license renewal ticket dedup for license #%d: %v", lic.ID, err)
			continue
		}
		if count > 0 {
			continue
		}
		var desc strings.Builder
		desc.WriteString("软件许可证即将到期，请及时办理续费。\n")
		desc.WriteString("- 软件名：" + lic.Name + "\n")
		if lic.Vendor != "" {
			desc.WriteString("- 厂商：" + lic.Vendor + "\n")
		}
		desc.WriteString("- 授权数量：" + strconv.Itoa(lic.TotalSeats) + "\n")
		if lic.ExpireDate != nil {
			desc.WriteString("- 到期日：" + lic.ExpireDate.Format("2006-01-02") + "\n")
		}
		ticket := model.Ticket{
			Type:        model.TicketTypeLicenseRenew,
			Title:       "软件许可续费：" + lic.Name,
			ApplicantID: admin.ID,
			LicenseID:   &lic.ID,
			AssetID:     lic.AssetID,
			Status:      model.TicketStatusDraft,
			Priority:    model.PriorityHigh,
			Description: desc.String(),
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&ticket).Error; err != nil {
				return err
			}
			return tx.Create(&model.TicketRecord{
				TicketID: ticket.ID,
				ActorID:  admin.ID,
				Action:   "create",
				ToStatus: model.TicketStatusDraft,
				Remark:   "许可证到期自动生成续费工单",
			}).Error
		}); err != nil {
			log.Printf("license renewal ticket create for license #%d: %v", lic.ID, err)
		}
	}
}

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

// WarrantyRemindDays 维保到期提醒窗口（默认未来 30 天内，含已过期）。
const WarrantyRemindDays = 30

// WarrantyReminderScheduler 维保到期提醒扫描器：每天检查一次，
// 对维保到期日在 [今天, 今天+30天] 或已过期的资产发送邮件（admin）与 IM 群通知。
type WarrantyReminderScheduler struct {
	db            *gorm.DB
	encryptionKey string
	lastDay       string
	stop          chan struct{}
	wg            sync.WaitGroup
	tasks         *TaskManager
}

// NewWarrantyReminderScheduler 创建维保提醒扫描器。
func NewWarrantyReminderScheduler(db *gorm.DB, encryptionKey string) *WarrantyReminderScheduler {
	return &WarrantyReminderScheduler{db: db, encryptionKey: encryptionKey}
}

func (s *WarrantyReminderScheduler) WithTaskManager(tasks *TaskManager) *WarrantyReminderScheduler {
	s.tasks = tasks
	return s
}

// Start 启动扫描循环（幂等）。
func (s *WarrantyReminderScheduler) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	if s.tasks != nil {
		s.tasks.Register("warranty_reminder", s.runTask)
		s.tasks.ResumeKind(context.Background(), "warranty_reminder")
	}
	s.wg.Add(1)
	go s.loop()
	log.Printf("warranty reminder scheduler started")
}

// Stop 停止扫描循环。
func (s *WarrantyReminderScheduler) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = nil
}

func (s *WarrantyReminderScheduler) loop() {
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

func (s *WarrantyReminderScheduler) tick() {
	today := time.Now().Format("2006-01-02")
	if s.lastDay == today {
		return // 每天只提醒一次
	}
	s.lastDay = today
	if s.tasks != nil {
		if _, err := s.tasks.Run(context.Background(), "warranty_reminder", "schedule",
			"warranty:"+today, map[string]interface{}{"date": today}); err != nil {
			log.Printf("warranty reminder task: %v", err)
		}
		return
	}
	assets := s.expiringAssets(time.Now())
	if len(assets) == 0 {
		return
	}
	s.notify(assets)
}

func (s *WarrantyReminderScheduler) runTask(_ context.Context, _ json.RawMessage) (interface{}, error) {
	assets := s.expiringAssets(time.Now())
	if len(assets) > 0 {
		s.notify(assets)
	}
	return map[string]interface{}{"count": len(assets)}, nil
}

// expiringAssets 查询维保到期日在 [now, now+30天] 或已过期的资产。
func (s *WarrantyReminderScheduler) expiringAssets(now time.Time) []model.Asset {
	deadline := now.AddDate(0, 0, WarrantyRemindDays)
	var assets []model.Asset
	if err := s.db.Where("warranty_expire_date IS NOT NULL AND warranty_expire_date <= ?", deadline).
		Order("warranty_expire_date asc").Find(&assets).Error; err != nil {
		log.Printf("warranty reminder: query assets: %v", err)
		return nil
	}
	return assets
}

func (s *WarrantyReminderScheduler) notify(assets []model.Asset) {
	// IM 群通知
	var b strings.Builder
	b.WriteString("- 维保即将到期资产共 " + strconv.Itoa(len(assets)) + " 台：\n")
	for _, a := range assets {
		expire := ""
		if a.WarrantyExpireDate != nil {
			expire = a.WarrantyExpireDate.Format("2006-01-02")
		}
		b.WriteString("- " + a.Hostname + " (" + a.IP + ")" + " 到期 " + expire + "\n")
	}
	if _, err := SendIMNotification(s.db, nil, s.encryptionKey, "维保到期提醒", BuildTicketMessage("维保到期提醒", b.String())); err != nil {
		log.Printf("warranty reminder: im notify: %v", err)
	}
	s.sendMail(assets)
}

func (s *WarrantyReminderScheduler) sendMail(assets []model.Asset) {
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
	b.WriteString("以下资产维保即将到期或已过期：\n\n")
	for _, a := range assets {
		expire := ""
		if a.WarrantyExpireDate != nil {
			expire = a.WarrantyExpireDate.Format("2006-01-02")
		}
		b.WriteString("- " + a.Hostname + " (" + a.IP + ") 维保到期 " + expire + "，厂商 " + a.MaintenanceVendor + "\n")
	}
	message := MailMessage{
		To:      recipients,
		Subject: "资产维保到期提醒（" + strconv.Itoa(len(assets)) + " 台）",
		Body:    b.String(),
	}
	if err := (SMTPMailSender{}).Send(cfg, password, message); err != nil {
		log.Printf("warranty reminder: mail notify: %v", err)
	}
}

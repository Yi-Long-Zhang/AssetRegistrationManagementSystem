package service

import (
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSLASchedulerTick(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Ticket{}, &model.TicketRecord{}, &model.MailConfig{}, &model.IMConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	user := model.User{Username: "sla-tester", Name: "SLA Tester", Role: model.RoleAdmin, Status: "active", PasswordHash: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	// 1) 审批阶段已超时（未通知）→ 应标记并生成审计
	overdue := model.Ticket{
		Type: model.TicketTypeMaintenance, Title: "超时审批单", ApplicantID: user.ID, ArchiveNo: "sla-test-1",
		Status: model.TicketStatusPendingApproval, SLAApprovalDeadline: &past,
	}
	// 2) 审批阶段未超时 → 不处理
	okTicket := model.Ticket{
		Type: model.TicketTypeMaintenance, Title: "未超时单", ApplicantID: user.ID, ArchiveNo: "sla-test-2",
		Status: model.TicketStatusPendingApproval, SLAApprovalDeadline: &future,
	}
	// 3) 执行阶段已超时（未通知）→ 应标记
	execOverdue := model.Ticket{
		Type: model.TicketTypeMaintenance, Title: "超时执行单", ApplicantID: user.ID, ArchiveNo: "sla-test-3",
		Status: model.TicketStatusInProgress, SLACompletionDeadline: &past,
	}
	// 4) 已通知过的超时 → 不重复处理
	notified := model.Ticket{
		Type: model.TicketTypeMaintenance, Title: "已通知单", ApplicantID: user.ID, ArchiveNo: "sla-test-4",
		Status: model.TicketStatusPendingApproval, SLAApprovalDeadline: &past, SLAOverdueNotified: true,
	}
	for _, tk := range []*model.Ticket{&overdue, &okTicket, &execOverdue, &notified} {
		if err := db.Create(tk).Error; err != nil {
			t.Fatalf("create ticket: %v", err)
		}
	}

	s := NewSLAScheduler(db, "")
	s.tick()

	var got model.Ticket
	if err := db.First(&got, overdue.ID).Error; err != nil {
		t.Fatalf("load overdue ticket: %v", err)
	}
	if !got.SLAOverdueNotified {
		t.Fatal("overdue pending ticket should be marked notified")
	}
	var gotExec model.Ticket
	if err := db.First(&gotExec, execOverdue.ID).Error; err != nil {
		t.Fatalf("load exec overdue ticket: %v", err)
	}
	if !gotExec.SLAOverdueNotified {
		t.Fatal("overdue in_progress ticket should be marked notified")
	}
	var ok model.Ticket
	if err := db.First(&ok, okTicket.ID).Error; err != nil {
		t.Fatalf("load ok ticket: %v", err)
	}
	if ok.SLAOverdueNotified {
		t.Fatal("non-overdue ticket must not be marked")
	}
	var noti model.Ticket
	if err := db.First(&noti, notified.ID).Error; err != nil {
		t.Fatalf("load notified ticket: %v", err)
	}
	if noti.SLAOverdueNotified != true {
		t.Fatal("notified ticket must keep flag")
	}

	// 审计记录：应恰好 2 条（两个首次超时的工单）
	var count int64
	if err := db.Model(&model.TicketRecord{}).Where("action = ?", "sla_overdue").Count(&count).Error; err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 2 {
		t.Fatalf("expect 2 sla_overdue records, got %d", count)
	}

	// 再次 tick：不应重复生成审计
	s.tick()
	if err := db.Model(&model.TicketRecord{}).Where("action = ?", "sla_overdue").Count(&count).Error; err != nil {
		t.Fatalf("count records again: %v", err)
	}
	if count != 2 {
		t.Fatalf("expect still 2 records after second tick, got %d", count)
	}
}

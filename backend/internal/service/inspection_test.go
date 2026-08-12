package service

import (
	"testing"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestNextInspectionRun(t *testing.T) {
	loc := time.UTC
	// 固定"今天 10:00"，规则 09:30 → 下次是明天 09:30
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, loc)
	daily := model.InspectionRule{Frequency: InspectionDaily, TimeOfDay: "09:30"}
	next, ok := NextInspectionRun(daily, now)
	if !ok {
		t.Fatal("daily should return next run")
	}
	if next.Format("2006-01-02 15:04") != "2026-08-13 09:30" {
		t.Fatalf("daily next = %s, want 2026-08-13 09:30", next.Format("2006-01-02 15:04"))
	}

	// 规则 11:00 → 下次就是今天 11:00
	daily2 := model.InspectionRule{Frequency: InspectionDaily, TimeOfDay: "11:00"}
	next, _ = NextInspectionRun(daily2, now)
	if next.Format("2006-01-02 15:04") != "2026-08-12 11:00" {
		t.Fatalf("daily next = %s, want 2026-08-12 11:00", next.Format("2006-01-02 15:04"))
	}

	// weekly：2026-08-12 是周三(3)，规则周四(4) 09:00 → 2026-08-13
	weekly := model.InspectionRule{Frequency: InspectionWeekly, DayOfWeek: 4, TimeOfDay: "09:00"}
	next, _ = NextInspectionRun(weekly, now)
	if next.Format("2006-01-02 15:04") != "2026-08-13 09:00" {
		t.Fatalf("weekly next = %s, want 2026-08-13 09:00", next.Format("2006-01-02 15:04"))
	}

	// monthly：每月 31 号，当前 8/12 10:00，8 月有 31 天 → 2026-08-31 09:00
	monthly := model.InspectionRule{Frequency: InspectionMonthly, DayOfMonth: 31, TimeOfDay: "09:00"}
	next, _ = NextInspectionRun(monthly, now)
	if next.Format("2006-01-02 15:04") != "2026-08-31 09:00" {
		t.Fatalf("monthly next = %s, want 2026-08-31 09:00", next.Format("2006-01-02 15:04"))
	}
	// 9 月只有 30 天：8/31 之后的下一次是 9/30（末月兜底）
	sepNow := time.Date(2026, 8, 31, 10, 0, 0, 0, loc)
	next, _ = NextInspectionRun(monthly, sepNow)
	if next.Format("2006-01-02 15:04") != "2026-09-30 09:00" {
		t.Fatalf("monthly next = %s, want 2026-09-30 09:00 (9月只有30天兜底)", next.Format("2006-01-02 15:04"))
	}

	// 非法时间 → 返回 false
	if _, ok := NextInspectionRun(model.InspectionRule{Frequency: InspectionDaily, TimeOfDay: "25:00"}, now); ok {
		t.Fatal("invalid time should return false")
	}
}

func TestInspectionSchedulerTick(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.InspectionRule{}, &model.Ticket{}, &model.TicketRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin := model.User{Username: "admin", Name: "Admin", Role: model.RoleAdmin, Status: "active", PasswordHash: "x"}
	exec := model.User{Username: "exec", Name: "Exec", Role: model.RoleAssetManager, Status: "active", PasswordHash: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.Create(&exec).Error; err != nil {
		t.Fatalf("create exec: %v", err)
	}

	// 执行时刻设为当前分钟（tick 时 now >= next）→ 触发
	pastRule := model.InspectionRule{
		Name: "每日巡检", Frequency: InspectionDaily, TimeOfDay: time.Now().Format("15:04"),
		AssigneeID: exec.ID, Enabled: true,
	}
	if err := db.Create(&pastRule).Error; err != nil {
		t.Fatalf("create rule: %v", err)
	}
	// 执行时刻设为两天后（今天周几+2）→ 未到不触发
	futureRule := model.InspectionRule{
		Name: "夜间巡检", Frequency: InspectionWeekly, DayOfWeek: (int(time.Now().Weekday()) + 2) % 7, TimeOfDay: "00:00",
		AssigneeID: exec.ID, Enabled: true,
	}
	if err := db.Create(&futureRule).Error; err != nil {
		t.Fatalf("create rule2: %v", err)
	}

	s := NewInspectionScheduler(db)
	s.tick()

	var count int64
	if err := db.Model(&model.Ticket{}).Where("type = ?", model.TicketTypeInspection).Count(&count).Error; err != nil {
		t.Fatalf("count tickets: %v", err)
	}
	if count != 1 {
		t.Fatalf("expect 1 inspection ticket, got %d", count)
	}
	var rule model.InspectionRule
	if err := db.First(&rule, pastRule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rule.LastRunAt == nil {
		t.Fatal("past rule last_run_at should be set")
	}
	var future model.InspectionRule
	if err := db.First(&future, futureRule.ID).Error; err != nil {
		t.Fatal(err)
	}
	if future.LastRunAt != nil {
		t.Fatal("future rule must not run before its time")
	}

	// 再次 tick：同一周期不应重复生成
	s.tick()
	if err := db.Model(&model.Ticket{}).Where("type = ?", model.TicketTypeInspection).Count(&count).Error; err != nil {
		t.Fatalf("count tickets again: %v", err)
	}
	if count != 1 {
		t.Fatalf("expect still 1 inspection ticket after second tick, got %d", count)
	}

	// 校验生成的工单：执行人为规则 AssigneeID
	var ticket model.Ticket
	if err := db.Where("type = ?", model.TicketTypeInspection).First(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if ticket.ExecutorID == nil || *ticket.ExecutorID != exec.ID {
		t.Fatalf("ticket executor should be %d, got %v", exec.ID, ticket.ExecutorID)
	}
	if ticket.Status != model.TicketStatusDraft {
		t.Fatalf("ticket status should be draft, got %s", ticket.Status)
	}
}

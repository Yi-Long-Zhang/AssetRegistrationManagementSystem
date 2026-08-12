package service

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// InspectionFrequency 巡检频率枚举。
const (
	InspectionDaily   = "daily"
	InspectionWeekly  = "weekly"
	InspectionMonthly = "monthly"
)

// NextInspectionRun 计算规则的下一次执行时间。
// now 之前的今天时刻视为已错过，跳到下一个周期。
func NextInspectionRun(rule model.InspectionRule, now time.Time) (time.Time, bool) {
	hour, minute, ok := parseHHMMTime(rule.TimeOfDay)
	if !ok {
		return time.Time{}, false
	}
	base := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	switch rule.Frequency {
	case InspectionDaily:
		if base.After(now) {
			return base, true
		}
		return base.AddDate(0, 0, 1), true
	case InspectionWeekly:
		day := int(now.Weekday())
		offset := (rule.DayOfWeek - day + 7) % 7
		next := base.AddDate(0, 0, offset)
		if next.Before(now) || next.Equal(now) {
			next = next.AddDate(0, 0, 7)
		}
		return next, true
	case InspectionMonthly:
		day := rule.DayOfMonth
		if day < 1 {
			day = 1
		}
		// 当月最后一天兜底（如 31 号在只有 30 天的月份）
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		if day > lastDay {
			day = lastDay
		}
		next := time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, now.Location())
		if next.Before(now) || next.Equal(now) {
			nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
			lastDay = time.Date(nextMonth.Year(), nextMonth.Month()+1, 0, 0, 0, 0, 0, nextMonth.Location()).Day()
			if day > lastDay {
				day = lastDay
			}
			next = time.Date(nextMonth.Year(), nextMonth.Month(), day, hour, minute, 0, 0, now.Location())
		}
		return next, true
	}
	return time.Time{}, false
}

func parseHHMMTime(value string) (hour, minute int, ok bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	m, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// CreateInspectionTicket 为巡检规则生成一张巡检工单（草稿，需人工提交审批）。
// 申请人取系统中首位 admin 用户；执行人取规则 AssigneeID。
func CreateInspectionTicket(db *gorm.DB, rule model.InspectionRule) (*model.Ticket, error) {
	var admin model.User
	if err := db.Where("role = ?", model.RoleAdmin).Order("id asc").First(&admin).Error; err != nil {
		return nil, err
	}
	ticket := model.Ticket{
		Type:        model.TicketTypeInspection,
		Title:       "定期巡检: " + rule.Name,
		ApplicantID: admin.ID,
		ExecutorID:  &rule.AssigneeID,
		Status:      model.TicketStatusDraft,
		Priority:    model.PriorityNormal,
		Description: buildInspectionDescription(rule),
		Remark:      "由巡检规则自动生成，请执行人按期完成巡检并填写结果。",
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ticket).Error; err != nil {
			return err
		}
		return tx.Create(&model.TicketRecord{
			TicketID: ticket.ID,
			ActorID:  admin.ID,
			Action:   "create",
			ToStatus: model.TicketStatusDraft,
			Remark:   "巡检规则自动生成工单",
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

func buildInspectionDescription(rule model.InspectionRule) string {
	if strings.TrimSpace(rule.Description) == "" {
		return fmt.Sprintf("按规则「%s」定期巡检，执行人：#%d", rule.Name, rule.AssigneeID)
	}
	return rule.Description
}

// InspectionScheduler 进程内巡检调度器：每分钟检查启用的巡检规则，
// 到达执行时刻且未生成过本周期工单时自动创建巡检工单（草稿）。
type InspectionScheduler struct {
	db   *gorm.DB
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewInspectionScheduler 创建巡检调度器。
func NewInspectionScheduler(db *gorm.DB) *InspectionScheduler {
	return &InspectionScheduler{db: db}
}

// Start 启动调度循环（幂等）。
func (s *InspectionScheduler) Start() {
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	s.wg.Add(1)
	go s.loop()
	log.Printf("inspection scheduler started")
}

// Stop 停止调度循环。
func (s *InspectionScheduler) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.stop = nil
}

func (s *InspectionScheduler) loop() {
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

func (s *InspectionScheduler) tick() {
	var rules []model.InspectionRule
	if err := s.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		log.Printf("inspection scheduler: load rules: %v", err)
		return
	}
	now := time.Now()
	for i := range rules {
		rule := rules[i]
		runAt, ok := cycleRunTime(rule, now)
		if !ok {
			continue
		}
		if now.Before(runAt) {
			continue // 未到本周期执行时刻
		}
		if rule.LastRunAt != nil && !rule.LastRunAt.Before(runAt) {
			continue // 本周期已生成
		}
		ticket, err := CreateInspectionTicket(s.db, rule)
		if err != nil {
			log.Printf("inspection scheduler: rule %d create ticket: %v", rule.ID, err)
			continue
		}
		now2 := time.Now()
		if err := s.db.Model(&rule).Update("last_run_at", now2).Error; err != nil {
			log.Printf("inspection scheduler: rule %d update last_run_at: %v", rule.ID, err)
		}
		log.Printf("inspection scheduler: rule %d created ticket #%d", rule.ID, ticket.ID)
	}
}

// cycleRunTime 计算规则在当前周期（今天/本周/本月）内的执行时刻。
// 本周/本月对应执行日已过时，返回下一周期的时刻（调度据此判断是否已到点）。
func cycleRunTime(rule model.InspectionRule, now time.Time) (time.Time, bool) {
	hour, minute, ok := parseHHMMTime(rule.TimeOfDay)
	if !ok {
		return time.Time{}, false
	}
	base := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	switch rule.Frequency {
	case InspectionDaily:
		return base, true
	case InspectionWeekly:
		day := int(now.Weekday())
		offset := (rule.DayOfWeek - day + 7) % 7
		return base.AddDate(0, 0, offset), true
	case InspectionMonthly:
		day := rule.DayOfMonth
		if day < 1 {
			day = 1
		}
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		if day > lastDay {
			day = lastDay
		}
		return time.Date(now.Year(), now.Month(), day, hour, minute, 0, 0, now.Location()), true
	}
	return time.Time{}, false
}

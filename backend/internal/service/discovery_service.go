package service

import (
	"context"
	"log"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// DiscoveryService 资产自动发现服务：执行扫描、比对台账并落库。
type DiscoveryService struct {
	DB          *gorm.DB
	Config      config.Config
	Runner      NmapRunner             // nil 时使用真实 exec 实现
	BinResolver func() (string, error) // nil 时使用默认探测（测试可注入固定路径）
	MailSender  MailSender             // nil 时使用 SMTPMailSender（告警邮件）
}

// NewDiscoveryService 创建发现服务。
func NewDiscoveryService(db *gorm.DB, cfg config.Config) *DiscoveryService {
	return &DiscoveryService{DB: db, Config: cfg, Runner: nil}
}

// Bin 解析 nmap 可执行文件路径（每次调用时解析，便于安装便携版后立即生效）。
func (s *DiscoveryService) Bin() (string, error) {
	if s.BinResolver != nil {
		return s.BinResolver()
	}
	return ResolveNmapBin(s.Config.Discovery.NmapBin)
}

// RunRule 同步执行一条规则的发现任务（StartRun + ExecuteRun），返回最终运行记录。
func (s *DiscoveryService) RunRule(ctx context.Context, ruleID uint, trigger string) (*model.DiscoveryRun, error) {
	run, err := s.StartRun(ruleID, trigger)
	if err != nil {
		return nil, err
	}
	if err := s.ExecuteRun(ctx, run.ID); err != nil {
		return run, err
	}
	var updated model.DiscoveryRun
	if err := s.DB.First(&updated, run.ID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}

// StartRun 创建一条 running 状态的发现运行记录。
func (s *DiscoveryService) StartRun(ruleID uint, trigger string) (*model.DiscoveryRun, error) {
	var rule model.DiscoveryRule
	if err := s.DB.First(&rule, ruleID).Error; err != nil {
		return nil, err
	}
	run := model.DiscoveryRun{
		RuleID:    rule.ID,
		Trigger:   trigger,
		Status:    model.DiscoveryRunStatusRunning,
		StartedAt: time.Now(),
	}
	if err := s.DB.Create(&run).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// ExecuteRun 执行运行记录对应的扫描、比对与落库，并更新记录状态。
func (s *DiscoveryService) ExecuteRun(ctx context.Context, runID uint) error {
	var run model.DiscoveryRun
	if err := s.DB.First(&run, runID).Error; err != nil {
		return err
	}
	var rule model.DiscoveryRule
	if err := s.DB.First(&rule, run.RuleID).Error; err != nil {
		return s.finishFailed(&run, err)
	}

	bin, err := s.Bin()
	if err != nil {
		return s.finishFailed(&run, err)
	}

	// 增量扫描：仅重扫上次成功运行中发现的存活主机（新主机需先跑全量或手动触发）
	var incrementalTargets []string
	if rule.Incremental {
		incrementalTargets = lastUpHosts(s.DB, rule.ID)
	}

	results, err := ScanTargets(ctx, s.Runner, bin, rule, s.Config.Discovery, incrementalTargets)
	if err != nil {
		return s.finishFailed(&run, err)
	}

	now := time.Now()
	if err := s.reconcile(&run, results, now); err != nil {
		return s.finishFailed(&run, err)
	}

	// 自动应用：规则开启 autoApply 时，仅自动应用低风险变更（changed-low / online），
	// 高风险（端口关闭/OS/主机名/服务变化）与离线保留人工确认。
	if rule.AutoApply {
		var autoHosts []model.DiscoveredHost
		if err := s.DB.Where("run_id = ? AND change_type IN ? AND change_risk = ?",
			run.ID,
			[]model.DiscoveryChangeType{model.DiscoveryChangeChanged, model.DiscoveryChangeOnline},
			model.ChangeRiskLow,
		).Find(&autoHosts).Error; err != nil {
			return s.finishFailed(&run, err)
		}
		var autoHostIDs []uint
		for i := range autoHosts {
			if autoHosts[i].MatchedAssetID != nil {
				autoHostIDs = append(autoHostIDs, autoHosts[i].ID)
			}
		}
		if len(autoHostIDs) > 0 {
			if _, err := s.ApplyHostChanges(ctx, run.ID, autoHostIDs, 0); err != nil {
				return s.finishFailed(&run, err)
			}
		}
	}

	run.Status = model.DiscoveryRunStatusSuccess
	run.FinishedAt = &now
	if err := s.DB.Save(&run).Error; err != nil {
		return err
	}
	s.DB.Model(&model.DiscoveryRule{}).Where("id = ?", rule.ID).Update("last_run_at", now)
	// 变更告警通知（邮件）；失败不影响主流程
	s.sendDiscoveryAlert(rule, &run)
	// 高风险变更自动生成工单（草稿）
	if rule.AutoTicket {
		if _, err := s.createAutoTickets(rule, &run); err != nil {
			log.Printf("discovery auto-ticket: %v", err)
		}
	}
	return nil
}

// finishFailed 将运行记录标记为失败并保存，返回错误。
func (s *DiscoveryService) finishFailed(run *model.DiscoveryRun, err error) error {
	now := time.Now()
	run.Status = model.DiscoveryRunStatusFailed
	run.Error = err.Error()
	run.FinishedAt = &now
	s.DB.Save(run)
	return err
}

// joinLines 将字符串切片合并为多行文本存储。
func joinLines(items []string) string {
	return strings.Join(items, "\n")
}

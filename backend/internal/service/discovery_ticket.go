package service

import (
	"asset-registration-management-system/backend/internal/model"

	"gorm.io/gorm"
)

// createAutoTickets 为运行结果中的高风险变更自动生成 IT 变更工单（草稿，需人工提交审批）。
// 申请人取系统中首位 admin 用户；仅对已匹配资产（MatchedAssetID 非空）的高风险 changed 主机生成。
// 返回生成数量。
func (s *DiscoveryService) createAutoTickets(rule model.DiscoveryRule, run *model.DiscoveryRun) (int, error) {
	var highHosts []model.DiscoveredHost
	if err := s.DB.Where("run_id = ? AND change_type = ? AND change_risk = ?",
		run.ID, model.DiscoveryChangeChanged, model.ChangeRiskHigh).Find(&highHosts).Error; err != nil {
		return 0, err
	}
	if len(highHosts) == 0 {
		return 0, nil
	}
	var admin model.User
	if err := s.DB.Where("role = ?", model.RoleAdmin).Order("id asc").First(&admin).Error; err != nil {
		return 0, err
	}

	created := 0
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		for i := range highHosts {
			h := &highHosts[i]
			if h.MatchedAssetID == nil {
				continue
			}
			ticket := model.Ticket{
				Type:            model.TicketTypeAssetChange,
				Title:           "自动变更工单: " + h.IP + " 高风险变更",
				ApplicantID:     admin.ID,
				AssetID:         h.MatchedAssetID,
				Status:          model.TicketStatusDraft,
				Priority:        model.PriorityHigh,
				Description:     "资产自动发现检测到高风险变更，需人工确认处理。\n差异摘要：" + h.DiffSummary,
				IPAddress:       h.IP,
				DeviceName:      h.Hostname,
				OpenPorts:       h.OpenPorts,
				RunningServices: h.Services,
			}
			if err := tx.Create(&ticket).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.TicketRecord{
				TicketID: ticket.ID,
				ActorID:  admin.ID,
				Action:   "create",
				ToStatus: model.TicketStatusDraft,
				Remark:   "自动发现高风险变更生成工单",
			}).Error; err != nil {
				return err
			}
			created++
		}
		return nil
	})
	return created, err
}

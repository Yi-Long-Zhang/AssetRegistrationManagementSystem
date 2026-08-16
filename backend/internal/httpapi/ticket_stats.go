package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ticketGroupedCounts 按字段分组统计工单数量。
type ticketGroupedCounts struct {
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// TicketStats 工单统计汇总。
type TicketStats struct {
	Total        int64                 `json:"total"`
	ByType       []ticketGroupedCounts `json:"byType"`
	ByStatus     []ticketGroupedCounts `json:"byStatus"`
	ByPriority   []ticketGroupedCounts `json:"byPriority"`
	MonthlyTrend []ticketGroupedCounts `json:"monthlyTrend"` // 近 12 个月（按创建月份）
	SLASummary   SLASummary            `json:"slaSummary"`
}

// SLASummary SLA 达标情况（针对已关闭且带 SLA 截止时间的工单）。
type SLASummary struct {
	Total      int64   `json:"total"`
	Met        int64   `json:"met"`
	Overdue    int64   `json:"overdue"`
	Rate       float64 `json:"rate"` // 达标率 0-100
	Applicable bool    `json:"applicable"`
}

// GetTicketStats 工单统计报表。
// @Summary 工单统计报表
// @Description 按类型/状态/优先级分布、月度趋势与 SLA 达标率统计（admin）
// @Tags tickets
// @Accept json
// @Produce json
// @Success 200 {object} TicketStats
// @Security BearerAuth
// @Router /tickets/stats [get]
func (h *Handler) GetTicketStats(c *gin.Context) {
	now := time.Now()
	// 每次统计都使用全新查询实例，避免 GORM 链式调用相互污染
	modelDB := func() *gorm.DB { return h.db.Model(&model.Ticket{}) }

	var total int64
	if err := modelDB().Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "统计工单失败")
		return
	}

	stats := TicketStats{
		Total:        total,
		ByType:       ticketGrouped(modelDB(), "type", 20),
		ByStatus:     ticketGrouped(modelDB(), "status", 20),
		ByPriority:   ticketGrouped(modelDB(), "priority", 10),
		MonthlyTrend: ticketMonthlyTrend(modelDB(), now),
		SLASummary:   ticketSLASummary(h.db),
	}
	c.JSON(http.StatusOK, stats)
}

// ExportTicketStats 导出工单统计报表 CSV。
// @Summary 导出工单统计报表
// @Description 导出工单统计汇总为 CSV（状态/类型/优先级/SLA 达标率/月度趋势，admin）
// @Tags tickets
// @Produce text/csv
// @Success 200 {string} string "CSV 文件"
// @Security BearerAuth
// @Router /tickets/stats/export [get]
func (h *Handler) ExportTicketStats(c *gin.Context) {
	stats := TicketStats{}
	now := time.Now()
	modelDB := func() *gorm.DB { return h.db.Model(&model.Ticket{}) }
	if err := modelDB().Count(&stats.Total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "导出统计失败")
		return
	}
	stats.ByType = ticketGrouped(modelDB(), "type", 20)
	stats.ByStatus = ticketGrouped(modelDB(), "status", 20)
	stats.ByPriority = ticketGrouped(modelDB(), "priority", 10)
	stats.MonthlyTrend = ticketMonthlyTrend(modelDB(), now)
	stats.SLASummary = ticketSLASummary(h.db)

	var buf strings.Builder
	writeCSVRow := func(row ...string) {
		for i, v := range row {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(csvEscape(v))
		}
		buf.WriteString("\n")
	}

	writeCSVRow("工单统计报表", "生成时间", now.Format("2006-01-02 15:04:05"))
	writeCSVRow("工单总数", fmt.Sprintf("%d", stats.Total), "")
	writeCSVRow()
	writeCSVRow("状态分布")
	writeCSVRow("状态", "数量")
	for _, item := range stats.ByStatus {
		writeCSVRow(ticketStatusLabel(item.Label), fmt.Sprintf("%d", item.Count))
	}
	writeCSVRow()
	writeCSVRow("类型分布")
	writeCSVRow("类型", "数量")
	for _, item := range stats.ByType {
		writeCSVRow(ticketTypeLabel(item.Label), fmt.Sprintf("%d", item.Count))
	}
	writeCSVRow()
	writeCSVRow("优先级分布")
	writeCSVRow("优先级", "数量")
	for _, item := range stats.ByPriority {
		writeCSVRow(item.Label, fmt.Sprintf("%d", item.Count))
	}
	writeCSVRow()
	writeCSVRow("SLA 达标率")
	writeCSVRow("指标", "数值")
	writeCSVRow("统计范围", "已关闭且配置 SLA 截止时间的工单")
	writeCSVRow("总数", fmt.Sprintf("%d", stats.SLASummary.Total))
	writeCSVRow("达标", fmt.Sprintf("%d", stats.SLASummary.Met))
	writeCSVRow("超时", fmt.Sprintf("%d", stats.SLASummary.Overdue))
	writeCSVRow("达标率", fmt.Sprintf("%.1f%%", stats.SLASummary.Rate))
	writeCSVRow()
	writeCSVRow("月度趋势（近 12 个月）")
	writeCSVRow("月份", "数量")
	for _, item := range stats.MonthlyTrend {
		writeCSVRow(item.Label, fmt.Sprintf("%d", item.Count))
	}

	c.Header("Content-Disposition", `attachment; filename="ticket-stats-`+now.Format("20060102")+`.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte("\ufeff"+buf.String()))
}

// ticketGrouped 按字段分组统计。
func ticketGrouped(db *gorm.DB, field string, limit int) []ticketGroupedCounts {
	var rows []struct {
		Key   string
		Count int64
	}
	if err := db.Select(field + " as key, count(*) as count").Group(field).Order("count desc").Limit(limit).Scan(&rows).Error; err != nil {
		return nil
	}
	items := make([]ticketGroupedCounts, 0, len(rows))
	for _, row := range rows {
		items = append(items, ticketGroupedCounts{Label: row.Key, Count: row.Count})
	}
	return items
}

// ticketMonthlyTrend 近 12 个月创建工单数（含当月），按月升序。
func ticketMonthlyTrend(db *gorm.DB, now time.Time) []ticketGroupedCounts {
	start := time.Date(now.Year(), now.Month()-11, 1, 0, 0, 0, 0, now.Location())
	items := make([]ticketGroupedCounts, 0, 12)
	for i := 0; i < 12; i++ {
		monthStart := start.AddDate(0, i, 0)
		monthEnd := monthStart.AddDate(0, 1, 0)
		var count int64
		_ = db.Where("created_at >= ? AND created_at < ?", monthStart, monthEnd).Count(&count).Error
		items = append(items, ticketGroupedCounts{Label: monthStart.Format("2006-01"), Count: count})
	}
	return items
}

// ticketSLASummary 已关闭且带 SLA 截止时间的工单达标统计。
func ticketSLASummary(db *gorm.DB) SLASummary {
	summary := SLASummary{}
	// 审批阶段截止与完成截止取其一（执行阶段截止优先）
	var rows []struct {
		SLACompletionDeadline *time.Time
		SLAApprovalDeadline   *time.Time
		ArchivedAt            *time.Time
	}
	if err := db.Model(&model.Ticket{}).
		Select("sla_completion_deadline, sla_approval_deadline, archived_at").
		Where("status = ? AND archived_at IS NOT NULL AND (sla_completion_deadline IS NOT NULL OR sla_approval_deadline IS NOT NULL)", model.TicketStatusClosed).
		Scan(&rows).Error; err != nil {
		return summary
	}
	summary.Total = int64(len(rows))
	for _, row := range rows {
		if row.ArchivedAt == nil {
			continue
		}
		deadline := row.SLACompletionDeadline
		if deadline == nil {
			deadline = row.SLAApprovalDeadline
		}
		if deadline != nil && row.ArchivedAt.Before(*deadline) {
			summary.Met++
		} else {
			summary.Overdue++
		}
	}
	if summary.Total > 0 {
		summary.Rate = float64(summary.Met) / float64(summary.Total) * 100
		summary.Applicable = true
	}
	return summary
}

func ticketStatusLabel(status string) string {
	labels := map[string]string{
		"draft": "草稿", "pending_approval": "审批中", "approved": "已审批",
		"rejected": "已驳回", "in_progress": "执行中", "pending_acceptance": "待验收",
		"closed": "已关闭", "cancelled": "已取消",
	}
	if label, ok := labels[status]; ok {
		return label
	}
	return status
}

func ticketTypeLabel(t string) string {
	labels := map[string]string{
		"asset_register": "资产登记", "asset_change": "资产变更",
		"asset_retire": "资产下线/报废", "maintenance": "权限/维护申请", "inspection": "定期巡检",
		"license_renew": "软件许可续费",
	}
	if label, ok := labels[t]; ok {
		return label
	}
	return t
}

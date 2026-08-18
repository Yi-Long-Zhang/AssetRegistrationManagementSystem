package httpapi

import (
	"net/http"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) findByID(c *gin.Context, id uint, out interface{}) bool {
	if err := h.db.First(out, id).Error; err != nil {
		statusForDBError(c, err, "资源不存在")
		return false
	}
	return true
}

func (h *Handler) findTicketForUser(c *gin.Context, id uint, ticket *model.Ticket) bool {
	if err := h.db.First(ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return false
	}
	return h.canViewTicket(c, *ticket)
}

func (h *Handler) canViewTicket(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	// 审批人：参与过任一审批节点的（含代理）可查看
	if user.Role == model.RoleApprover {
		var count int64
		_ = h.db.Model(&model.TicketWorkflowStepApprover{}).
			Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.id = ticket_workflow_step_approvers.step_id").
			Joins("JOIN users ON users.id = ticket_workflow_step_approvers.user_id").
			Where("ticket_workflow_steps.ticket_id = ? AND (ticket_workflow_step_approvers.user_id = ? OR users.proxy_user_id = ?)", ticket.ID, user.ID, user.ID).
			Count(&count).Error
		if count > 0 {
			return true
		}
	}
	// 资产管理员：执行阶段及之后可查看
	if user.Role == model.RoleAssetManager {
		return ticket.Status == model.TicketStatusApproved ||
			ticket.Status == model.TicketStatusInProgress ||
			ticket.Status == model.TicketStatusPendingAcceptance ||
			ticket.Status == model.TicketStatusClosed
	}
	errorJSON(c, http.StatusForbidden, "没有权限查看该工单")
	return false
}

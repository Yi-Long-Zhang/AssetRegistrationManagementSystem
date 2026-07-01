package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	workflowStepPending  = "pending"
	workflowStepApproved = "approved"
	workflowStepRejected = "rejected"
)

func (h *Handler) ListWorkflows(c *gin.Context) {
	var workflows []model.TicketWorkflow
	if err := h.db.Preload("Nodes.Approvers.User").Order("type asc").Find(&workflows).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询流程配置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": workflows})
}

func (h *Handler) GetWorkflow(c *gin.Context) {
	workflow, ok := h.workflowByType(c, model.TicketType(c.Param("type")))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, workflow)
}

func (h *Handler) SaveWorkflow(c *gin.Context) {
	ticketType := model.TicketType(c.Param("type"))
	var req workflowRequest
	if !bind(c, &req) {
		return
	}
	if len(req.Nodes) == 0 {
		errorJSON(c, http.StatusBadRequest, "流程至少需要一个审批节点")
		return
	}
	for _, node := range req.Nodes {
		if strings.TrimSpace(node.Name) == "" || len(node.ApproverIDs) == 0 {
			errorJSON(c, http.StatusBadRequest, "审批节点名称和审批人不能为空")
			return
		}
		var count int64
		if err := h.db.Model(&model.User{}).Where("id IN ? AND status = ?", node.ApproverIDs, "active").Count(&count).Error; err != nil || count != int64(len(uniqueUint(node.ApproverIDs))) {
			errorJSON(c, http.StatusBadRequest, "审批人必须是有效启用用户")
			return
		}
	}

	var workflow model.TicketWorkflow
	err := h.db.Where("type = ?", ticketType).First(&workflow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		workflow = model.TicketWorkflow{Type: ticketType}
	} else if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询流程配置失败")
		return
	}
	workflow.Name = defaultString(req.Name, string(ticketType)+" 流程")
	workflow.Enabled = req.Enabled
	if workflow.Name == "" {
		workflow.Name = string(ticketType) + " 流程"
	}
	if err := h.db.Save(&workflow).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存流程配置失败: "+err.Error())
		return
	}
	if err := h.db.Where("workflow_id = ?", workflow.ID).Delete(&model.TicketWorkflowNode{}).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "清理旧流程节点失败: "+err.Error())
		return
	}
	for index, nodeReq := range req.Nodes {
		node := model.TicketWorkflowNode{WorkflowID: workflow.ID, Name: strings.TrimSpace(nodeReq.Name), SortOrder: index + 1}
		if err := h.db.Create(&node).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "保存流程节点失败: "+err.Error())
			return
		}
		for _, userID := range uniqueUint(nodeReq.ApproverIDs) {
			if err := h.db.Create(&model.TicketWorkflowNodeApprover{NodeID: node.ID, UserID: userID}).Error; err != nil {
				errorJSON(c, http.StatusBadRequest, "保存节点审批人失败: "+err.Error())
				return
			}
		}
	}
	h.db.Preload("Nodes.Approvers.User").First(&workflow, workflow.ID)
	c.JSON(http.StatusOK, workflow)
}

func (h *Handler) EnableWorkflow(c *gin.Context) {
	workflow, ok := h.workflowByType(c, model.TicketType(c.Param("type")))
	if !ok {
		return
	}
	workflow.Enabled = true
	if err := h.db.Save(&workflow).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "启用流程失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, workflow)
}

func (h *Handler) workflowByType(c *gin.Context, ticketType model.TicketType) (model.TicketWorkflow, bool) {
	var workflow model.TicketWorkflow
	err := h.db.Preload("Nodes.Approvers.User", func(db *gorm.DB) *gorm.DB {
		return db.Order("id asc")
	}).Preload("Nodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("type = ?", ticketType).First(&workflow).Error
	if err != nil {
		statusForDBError(c, err, "流程配置不存在")
		return model.TicketWorkflow{}, false
	}
	return workflow, true
}

func (h *Handler) createWorkflowSnapshot(c *gin.Context, ticket *model.Ticket) bool {
	var workflow model.TicketWorkflow
	err := h.db.Preload("Nodes.Approvers", func(db *gorm.DB) *gorm.DB {
		return db.Order("id asc")
	}).Preload("Nodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order asc")
	}).Where("type = ? AND enabled = ?", ticket.Type, true).First(&workflow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorJSON(c, http.StatusBadRequest, "该工单类型尚未配置启用的审批流程")
			return false
		}
		errorJSON(c, http.StatusInternalServerError, "查询审批流程失败")
		return false
	}
	if len(workflow.Nodes) == 0 {
		errorJSON(c, http.StatusBadRequest, "该工单类型审批流程没有审批节点")
		return false
	}
	if err := h.db.Where("ticket_id = ?", ticket.ID).Delete(&model.TicketWorkflowStep{}).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "重置流程快照失败: "+err.Error())
		return false
	}
	for _, node := range workflow.Nodes {
		step := model.TicketWorkflowStep{TicketID: ticket.ID, Name: node.Name, SortOrder: node.SortOrder, Status: workflowStepPending}
		if err := h.db.Create(&step).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "创建审批节点失败: "+err.Error())
			return false
		}
		for _, approver := range node.Approvers {
			if err := h.db.Create(&model.TicketWorkflowStepApprover{StepID: step.ID, UserID: approver.UserID}).Error; err != nil {
				errorJSON(c, http.StatusBadRequest, "创建审批人快照失败: "+err.Error())
				return false
			}
		}
	}
	return h.activateNextWorkflowStep(c, ticket)
}

func (h *Handler) approveCurrentStep(c *gin.Context, ticket *model.Ticket, user model.User, remark string) bool {
	var step model.TicketWorkflowStep
	if !h.currentStep(c, ticket, &step) {
		return false
	}
	if user.Role != model.RoleAdmin && !h.isStepApprover(step.ID, user.ID) {
		errorJSON(c, http.StatusForbidden, "只有当前审批节点审批人可以处理")
		return false
	}
	now := time.Now()
	step.Status = workflowStepApproved
	step.ActorID = &user.ID
	step.Remark = remark
	step.ActedAt = &now
	if err := h.db.Save(&step).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存审批结果失败: "+err.Error())
		return false
	}
	h.addRecord(ticket.ID, user.ID, "approve:"+step.Name, ticket.Status, ticket.Status, remark)
	return h.activateNextWorkflowStep(c, ticket)
}

func (h *Handler) rejectCurrentStep(c *gin.Context, ticket *model.Ticket, user model.User, remark string) bool {
	var step model.TicketWorkflowStep
	if !h.currentStep(c, ticket, &step) {
		return false
	}
	if user.Role != model.RoleAdmin && !h.isStepApprover(step.ID, user.ID) {
		errorJSON(c, http.StatusForbidden, "只有当前审批节点审批人可以处理")
		return false
	}
	now := time.Now()
	step.Status = workflowStepRejected
	step.ActorID = &user.ID
	step.Remark = remark
	step.ActedAt = &now
	if err := h.db.Save(&step).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存驳回结果失败: "+err.Error())
		return false
	}
	ticket.CurrentWorkflowStepID = nil
	ticket.CurrentWorkflowStepName = ""
	ticket.ApproverID = &user.ID
	if err := h.db.Save(ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新工单状态失败: "+err.Error())
		return false
	}
	return true
}

func (h *Handler) activateNextWorkflowStep(c *gin.Context, ticket *model.Ticket) bool {
	var step model.TicketWorkflowStep
	err := h.db.Preload("Approvers").Where("ticket_id = ? AND status = ?", ticket.ID, workflowStepPending).Order("sort_order asc").First(&step).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ticket.CurrentWorkflowStepID = nil
		ticket.CurrentWorkflowStepName = ""
		ticket.Status = model.TicketStatusApproved
		if err := h.db.Save(ticket).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "更新工单审批完成状态失败: "+err.Error())
			return false
		}
		return true
	}
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询下一审批节点失败")
		return false
	}
	ticket.CurrentWorkflowStepID = &step.ID
	ticket.CurrentWorkflowStepName = step.Name
	ticket.Status = model.TicketStatusPendingApproval
	if len(step.Approvers) > 0 {
		ticket.ApproverID = &step.Approvers[0].UserID
	}
	if err := h.db.Save(ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新当前审批节点失败: "+err.Error())
		return false
	}
	return true
}

func (h *Handler) currentStep(c *gin.Context, ticket *model.Ticket, out *model.TicketWorkflowStep) bool {
	if ticket.CurrentWorkflowStepID == nil {
		errorJSON(c, http.StatusBadRequest, "工单没有当前审批节点")
		return false
	}
	if err := h.db.First(out, *ticket.CurrentWorkflowStepID).Error; err != nil {
		statusForDBError(c, err, "当前审批节点不存在")
		return false
	}
	return true
}

func (h *Handler) isStepApprover(stepID, userID uint) bool {
	var count int64
	_ = h.db.Model(&model.TicketWorkflowStepApprover{}).Where("step_id = ? AND user_id = ?", stepID, userID).Count(&count).Error
	return count > 0
}

func uniqueUint(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

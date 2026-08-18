package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) TicketAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req ticketActionRequest
		_ = c.ShouldBindJSON(&req)

		user := currentUser(c)
		var ticket model.Ticket
		if !h.findTicketForUser(c, id, &ticket) {
			return
		}
		next, err := service.Transition(action, ticket.Status, user.Role)
		if err != nil {
			errorJSON(c, http.StatusForbidden, err.Error())
			return
		}
		if (action == "submit" || action == "accept") && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
			errorJSON(c, http.StatusForbidden, "只能提交自己的工单")
			return
		}
		if action == "submit" {
			if !h.createWorkflowSnapshot(c, &ticket) {
				return
			}
			h.addRecord(ticket.ID, user.ID, action, model.TicketStatusDraft, ticket.Status, req.Remark)
			h.notifyCurrentApprovers(ticket.ID)
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "withdraw" {
			// 仅申请人本人或管理员可撤回；回到草稿后可修改并重新提交
			if ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
				errorJSON(c, http.StatusForbidden, "只能撤回自己提交的工单")
				return
			}
			from := ticket.Status
			ticket.Status = next
			// 重置审批流程快照，清理当前节点与 SLA 审批截止时间
			if err := h.db.Where("ticket_id = ?", ticket.ID).Delete(&model.TicketWorkflowStep{}).Error; err != nil {
				errorJSON(c, http.StatusBadRequest, "重置流程快照失败: "+err.Error())
				return
			}
			ticket.CurrentWorkflowStepID = nil
			ticket.CurrentWorkflowStepName = ""
			ticket.SLAApprovalDeadline = nil
			if err := h.db.Save(&ticket).Error; err != nil {
				errorJSON(c, http.StatusBadRequest, "撤回工单失败: "+err.Error())
				return
			}
			h.addRecord(ticket.ID, user.ID, "withdraw", from, next, req.Remark)
			h.notifyTicketIM(&ticket, "工单已撤回",
				fmt.Sprintf("- 工单：#%d %s\n- 撤回人：%s", ticket.ID, ticket.Title, user.Username))
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "approve" {
			if !h.approveCurrentStep(c, &ticket, user, req.Remark) {
				return
			}
			h.notifyTicketIM(&ticket, "工单审批通过",
				fmt.Sprintf("- 工单：#%d %s\n- 审批人：%s", ticket.ID, ticket.Title, user.Username))
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "reject" {
			from := ticket.Status
			ticket.Status = next
			if !h.rejectCurrentStep(c, &ticket, user, req.Remark) {
				return
			}
			h.addRecord(ticket.ID, user.ID, action, from, next, req.Remark)
			h.notifyTicketIM(&ticket, "工单被驳回",
				fmt.Sprintf("- 工单：#%d %s\n- 审批人：%s\n- 原因：%s", ticket.ID, ticket.Title, user.Username, req.Remark))
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "accept" && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
			errorJSON(c, http.StatusForbidden, "只有申请人可以验收关闭该工单")
			return
		}

		from := ticket.Status
		ticket.Status = next
		if action == "start" {
			ticket.ExecutorID = &user.ID
			// SLA：进入执行阶段时按流程类型写入完成截止时间
			var wf model.TicketWorkflow
			if err := h.db.Where("type = ?", ticket.Type).First(&wf).Error; err == nil && wf.CompletionHours != nil && *wf.CompletionHours > 0 {
				now := time.Now()
				deadline := now.Add(time.Duration(*wf.CompletionHours) * time.Hour)
				ticket.SLACompletionDeadline = &deadline
				ticket.SLAStartedAt = &now
			}
		}
		if action == "complete" {
			ticket.Result = req.Result
		}
		if action == "accept" {
			ticket.AcceptanceResult = defaultString(req.AcceptanceResult, req.Remark)
			archiveNo, archivePath, ok := h.closeTicketWithArchive(c, &ticket)
			if !ok {
				return
			}
			ticket.ArchiveNo = &archiveNo
			ticket.ArchivePath = archivePath
			now := time.Now()
			ticket.ArchivedAt = &now
		}
		if err := h.db.Save(&ticket).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "更新工单状态失败: "+err.Error())
			return
		}
		h.addRecord(ticket.ID, user.ID, action, from, next, req.Remark)
		if action == "accept" && next == model.TicketStatusClosed {
			h.notifyTicketIM(&ticket, "工单已验收关闭",
				fmt.Sprintf("- 工单：#%d %s\n- 归档号：%s\n- 验收结果：%s",
					ticket.ID, ticket.Title, derefStr(ticket.ArchiveNo), ticket.AcceptanceResult))
		}
		if action == "submit" && next == model.TicketStatusPendingApproval {
			h.notifyTicketIM(&ticket, "工单待审批",
				fmt.Sprintf("- 工单：#%d %s\n- 申请人：%s\n- 请审批人尽快处理", ticket.ID, ticket.Title, user.Username))
		}
		c.JSON(http.StatusOK, ticket)
	}
}

type ticketTransferRequest struct {
	ToUserID uint `json:"toUserId" binding:"required"`
}

// TransferTicketApprover 审批转交：当前节点审批人或 admin 将当前审批节点转交给其他用户。
func (h *Handler) TransferTicketApprover(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req ticketTransferRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	if user.Role != model.RoleAdmin && user.Role != model.RoleApprover {
		errorJSON(c, http.StatusForbidden, "没有权限执行该操作")
		return
	}
	var ticket model.Ticket
	if !h.findTicketForUser(c, id, &ticket) {
		return
	}
	if ticket.Status != model.TicketStatusPendingApproval {
		errorJSON(c, http.StatusBadRequest, "只有审批中的工单可以转交")
		return
	}
	var target model.User
	if err := h.db.First(&target, req.ToUserID).Error; err != nil {
		statusForDBError(c, err, "目标用户不存在")
		return
	}
	if target.Status != "active" {
		errorJSON(c, http.StatusBadRequest, "目标用户未启用")
		return
	}
	var step model.TicketWorkflowStep
	if !h.currentStep(c, &ticket, &step) {
		return
	}
	if user.Role != model.RoleAdmin && !h.isStepApprover(step.ID, user.ID) {
		errorJSON(c, http.StatusForbidden, "只有当前节点审批人可以转交")
		return
	}
	// 替换当前节点审批人为目标用户
	if err := h.db.Where("step_id = ?", step.ID).Delete(&model.TicketWorkflowStepApprover{}).Error; err != nil {
		log.Printf("transfer ticket: %v", err)
		errorJSON(c, http.StatusBadRequest, "转交失败")
		return
	}
	if err := h.db.Create(&model.TicketWorkflowStepApprover{StepID: step.ID, UserID: target.ID}).Error; err != nil {
		log.Printf("transfer ticket: %v", err)
		errorJSON(c, http.StatusBadRequest, "转交失败")
		return
	}
	h.addRecord(ticket.ID, user.ID, "transfer", ticket.Status, ticket.Status, "审批转交: "+target.Name)
	h.notifyTicketIM(&ticket, "工单审批已转交",
		fmt.Sprintf("- 工单：#%d %s\n- 转交人：%s → %s", ticket.ID, ticket.Title, user.Name, target.Name))
	c.JSON(http.StatusOK, ticket)
}

type ticketBatchApproveRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Remark string `json:"remark"`
}

// BatchApproveTickets 批量审批：逐单校验（审批中 + 当前节点审批人/admin），
// 成功单生效，无权/非审批中的单计入 skipped 返回。
func (h *Handler) BatchApproveTickets(c *gin.Context) {
	var req ticketBatchApproveRequest
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		errorJSON(c, http.StatusBadRequest, "未选择工单")
		return
	}
	if len(req.IDs) > 100 {
		errorJSON(c, http.StatusBadRequest, "单次最多审批 100 张工单")
		return
	}
	user := currentUser(c)
	if user.Role != model.RoleAdmin && user.Role != model.RoleApprover {
		errorJSON(c, http.StatusForbidden, "没有权限执行该操作")
		return
	}
	approved := 0
	var skipped []uint
	for _, id := range req.IDs {
		var ticket model.Ticket
		if err := h.db.First(&ticket, id).Error; err != nil {
			skipped = append(skipped, id)
			continue
		}
		if ticket.Status != model.TicketStatusPendingApproval {
			skipped = append(skipped, id)
			continue
		}
		if user.Role != model.RoleAdmin {
			var step model.TicketWorkflowStep
			if err := h.db.Where("ticket_id = ? AND status = ?", ticket.ID, workflowStepPending).Order("sort_order asc").First(&step).Error; err != nil || !h.isStepApprover(step.ID, user.ID) {
				skipped = append(skipped, id)
				continue
			}
		}
		if !h.approveSingleSilent(&ticket, user, req.Remark) {
			skipped = append(skipped, id)
			continue
		}
		approved++
	}
	c.JSON(http.StatusOK, gin.H{"approved": approved, "skipped": skipped})
}

// approveSingleSilent 单工单审批（批量场景用，不向响应写错误）。
func (h *Handler) approveSingleSilent(ticket *model.Ticket, user model.User, remark string) bool {
	var step model.TicketWorkflowStep
	if err := h.db.Where("ticket_id = ? AND status = ?", ticket.ID, workflowStepPending).Order("sort_order asc").First(&step).Error; err != nil {
		return false
	}
	now := time.Now()
	step.Status = workflowStepApproved
	step.ActorID = &user.ID
	step.Remark = remark
	step.ActedAt = &now
	if err := h.db.Save(&step).Error; err != nil {
		return false
	}
	h.addRecord(ticket.ID, user.ID, "approve:"+step.Name, ticket.Status, ticket.Status, remark)
	return h.activateNextStepSilent(ticket)
}

// activateNextStepSilent 推进到下一审批节点（批量场景用，不向响应写错误）。
func (h *Handler) activateNextStepSilent(ticket *model.Ticket) bool {
	var step model.TicketWorkflowStep
	err := h.db.Preload("Approvers").Where("ticket_id = ? AND status = ?", ticket.ID, workflowStepPending).Order("sort_order asc").First(&step).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ticket.CurrentWorkflowStepID = nil
		ticket.CurrentWorkflowStepName = ""
		ticket.Status = model.TicketStatusApproved
		return h.db.Save(ticket).Error == nil
	}
	if err != nil || len(step.Approvers) == 0 {
		return false
	}
	ticket.CurrentWorkflowStepID = &step.ID
	ticket.CurrentWorkflowStepName = step.Name
	ticket.Status = model.TicketStatusPendingApproval
	return h.db.Save(ticket).Error == nil
}

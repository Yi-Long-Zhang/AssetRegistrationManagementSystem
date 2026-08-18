package httpapi

import (
	"errors"
	"net/http"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) ListTicketTypeApprovers(c *gin.Context) {
	var items []model.TicketTypeApprover
	if err := h.db.Preload("Approver").Order("type asc").Find(&items).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询审批配置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) SetTicketTypeApprover(c *gin.Context) {
	ticketType := model.TicketType(c.Param("type"))
	var req ticketTypeApproverRequest
	if !bind(c, &req) {
		return
	}
	var approver model.User
	if err := h.db.First(&approver, req.ApproverID).Error; err != nil {
		statusForDBError(c, err, "审批人不存在")
		return
	}
	if approver.Role != model.RoleApprover && approver.Role != model.RoleAdmin {
		errorJSON(c, http.StatusBadRequest, "审批人必须是审批人或管理员角色")
		return
	}

	var item model.TicketTypeApprover
	err := h.db.Where("type = ?", ticketType).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = model.TicketTypeApprover{Type: ticketType, ApproverID: req.ApproverID}
		err = h.db.Create(&item).Error
	} else if err == nil {
		item.ApproverID = req.ApproverID
		err = h.db.Save(&item).Error
	}
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "保存审批配置失败: "+err.Error())
		return
	}
	h.db.Preload("Approver").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

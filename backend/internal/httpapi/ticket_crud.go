package httpapi

import (
	"fmt"
	"log"
	"net/http"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) ListTickets(c *gin.Context) {
	user := currentUser(c)
	var tickets []model.Ticket
	db := h.db.Preload("Applicant").Preload("Asset").Preload("WorkflowSteps.Approvers").Order("tickets.id desc")
	switch c.Query("view") {
	case "todo":
		db = h.applyTodoFilter(db, user)
	case "submitted":
		db = db.Where("applicant_id = ?", user.ID)
	default:
		if user.Role == model.RoleApplicant {
			db = db.Where("applicant_id = ?", user.ID)
		}
	}
	if user.Role == model.RoleApplicant && c.Query("view") == "all" {
		db = db.Where("applicant_id = ?", user.ID)
	}
	if err := db.Find(&tickets).Error; err != nil {
		log.Printf("list tickets: %v", err)
		errorJSON(c, http.StatusInternalServerError, "查询工单失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tickets})
}

func (h *Handler) CreateTicket(c *gin.Context) {
	var req ticketRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	ticket := model.Ticket{
		Type:            req.Type,
		Title:           req.Title,
		ApplicantID:     user.ID,
		AssetID:         req.AssetID,
		Status:          model.TicketStatusDraft,
		Priority:        defaultPriority(req.Priority),
		Description:     req.Description,
		DeviceType:      req.DeviceType,
		DeviceName:      req.DeviceName,
		IPAddress:       req.IPAddress,
		OpenPorts:       req.OpenPorts,
		RunningServices: req.RunningServices,
		AppVersion:      req.AppVersion,
		Manufacturer:    req.Manufacturer,
		Antivirus:       req.Antivirus,
		ChangeContent:   req.ChangeContent,
		Impact:          req.Impact,
		Remark:          req.Remark,
	}
	if err := h.db.Create(&ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建工单失败: "+err.Error())
		return
	}
	if err := h.saveTicketAssets(ticket.ID, req.AssetIDs, req.AssetID); err != nil {
		errorJSON(c, http.StatusBadRequest, "保存关联资产失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "create", "", ticket.Status, "创建工单")
	h.notifyTicketIM(&ticket, "新工单创建",
		fmt.Sprintf("- 工单：#%d %s\n- 类型：%s\n- 申请人：%s\n- 优先级：%s",
			ticket.ID, ticket.Title, ticket.Type, user.Username, ticket.Priority))
	c.JSON(http.StatusCreated, ticket)
}

// notifyTicketIM 发送工单事件群通知；未配置或发送失败仅记日志，不影响业务。
func (h *Handler) notifyTicketIM(ticket *model.Ticket, event, detail string) {
	if _, err := service.SendIMNotification(h.db, h.im, h.cfg.Security.ConfigEncryptionKey,
		"工单通知: "+ticket.Title, service.BuildTicketMessage(event, detail)); err != nil {
		log.Printf("IM notify ticket #%d: %v", ticket.ID, err)
	}
}

func (h *Handler) GetTicket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Assets.Asset").Preload("Approver").Preload("Executor").
		Preload("Records.Actor").Preload("Comments.Actor").Preload("Attachments.Uploader").
		Preload("WorkflowSteps.Actor").Preload("WorkflowSteps.Approvers.User", func(db *gorm.DB) *gorm.DB { return db.Order("id asc") }).
		Preload("WorkflowSteps", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc") }).
		First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return
	}
	if !h.canViewTicket(c, ticket) {
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *Handler) UpdateTicket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if !h.findTicketForUser(c, id, &ticket) {
		return
	}
	if ticket.Status != model.TicketStatusDraft && ticket.Status != model.TicketStatusRejected {
		errorJSON(c, http.StatusBadRequest, "只有草稿或驳回工单可以编辑")
		return
	}
	var req ticketRequest
	if !bind(c, &req) {
		return
	}
	applyTicketRequest(&ticket, req)
	if err := h.db.Save(&ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新工单失败: "+err.Error())
		return
	}
	if err := h.saveTicketAssets(ticket.ID, req.AssetIDs, req.AssetID); err != nil {
		errorJSON(c, http.StatusBadRequest, "保存关联资产失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, ticket)
}

// saveTicketAssets 保存工单关联资产（先删后插）。
// assetIDs 优先；为空时回退到单资产 assetID（兼容旧调用）。
func (h *Handler) saveTicketAssets(ticketID uint, assetIDs []uint, assetID *uint) error {
	ids := uniqueUint(assetIDs)
	if len(ids) == 0 && assetID != nil && *assetID > 0 {
		ids = []uint{*assetID}
	}
	if err := h.db.Where("ticket_id = ?", ticketID).Delete(&model.TicketAsset{}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := h.db.Create(&model.TicketAsset{TicketID: ticketID, AssetID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

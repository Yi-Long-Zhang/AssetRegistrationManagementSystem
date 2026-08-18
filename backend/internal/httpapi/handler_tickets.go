package httpapi

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

func (h *Handler) DownloadTicketArchive(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return
	}
	if !h.canDownloadArchive(c, ticket) {
		return
	}
	if ticket.Status != model.TicketStatusClosed {
		errorJSON(c, http.StatusBadRequest, "工单关闭后才能下载归档 PDF")
		return
	}
	if ticket.ArchivePath == "" || ticket.ArchiveNo == nil || *ticket.ArchiveNo == "" {
		errorJSON(c, http.StatusBadRequest, "工单归档 PDF 尚未生成")
		return
	}
	c.FileAttachment(ticket.ArchivePath, *ticket.ArchiveNo+".pdf")
}

func (h *Handler) DownloadTicketArchives(c *gin.Context) {
	var req ticketArchiveBatchRequest
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		errorJSON(c, http.StatusBadRequest, "请选择要下载的工单")
		return
	}
	if len(req.IDs) > 100 {
		errorJSON(c, http.StatusBadRequest, "单次最多下载 100 个归档 PDF")
		return
	}
	ids := uniqueUint(req.IDs)
	var tickets []model.Ticket
	if err := h.db.Where("id IN ?", ids).Find(&tickets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询工单归档失败")
		return
	}
	if len(tickets) != len(ids) {
		errorJSON(c, http.StatusNotFound, "包含不存在的工单")
		return
	}
	ticketByID := make(map[uint]model.Ticket, len(tickets))
	for _, ticket := range tickets {
		ticketByID[ticket.ID] = ticket
	}
	ordered := make([]model.Ticket, 0, len(ids))
	for _, id := range ids {
		ticket := ticketByID[id]
		if !h.canDownloadArchive(c, ticket) {
			return
		}
		if ticket.Status != model.TicketStatusClosed {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 关闭后才能下载归档 PDF", ticket.ID))
			return
		}
		if ticket.ArchivePath == "" || ticket.ArchiveNo == nil || *ticket.ArchiveNo == "" {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 归档 PDF 尚未生成", ticket.ID))
			return
		}
		if _, err := os.Stat(ticket.ArchivePath); err != nil {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 归档 PDF 文件不存在", ticket.ID))
			return
		}
		ordered = append(ordered, ticket)
	}

	filename := fmt.Sprintf("ticket-archives-%s.zip", time.Now().Format("20060102150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()
	usedNames := map[string]int{}
	for _, ticket := range ordered {
		name := *ticket.ArchiveNo + ".pdf"
		if usedNames[name] > 0 {
			name = fmt.Sprintf("%s-%d.pdf", *ticket.ArchiveNo, usedNames[name]+1)
		}
		usedNames[*ticket.ArchiveNo+".pdf"]++
		if err := addFileToZip(zipWriter, ticket.ArchivePath, name); err != nil {
			errorJSON(c, http.StatusInternalServerError, "打包归档 PDF 失败: "+err.Error())
			return
		}
	}
}

func (h *Handler) ListTicketComments(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var comments []model.TicketComment
	if err := h.db.Preload("Actor").Where("ticket_id = ?", ticket.ID).Order("id asc").Find(&comments).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询评论失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": comments})
}

func (h *Handler) CreateTicketComment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var req ticketCommentRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	comment := model.TicketComment{TicketID: ticket.ID, ActorID: user.ID, Content: strings.TrimSpace(req.Content)}
	if comment.Content == "" {
		errorJSON(c, http.StatusBadRequest, "评论不能为空")
		return
	}
	if err := h.db.Create(&comment).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建评论失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "comment", ticket.Status, ticket.Status, comment.Content)
	h.db.Preload("Actor").First(&comment, comment.ID)
	c.JSON(http.StatusCreated, comment)
}

func (h *Handler) ListTicketAttachments(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var attachments []model.TicketAttachment
	if err := h.db.Preload("Uploader").Where("ticket_id = ?", ticket.ID).Order("id desc").Find(&attachments).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询附件失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": attachments})
}

func (h *Handler) UploadTicketAttachment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "请选择上传文件")
		return
	}
	const maxUploadSize = 20 << 20 // 20MB
	if file.Size > maxUploadSize {
		errorJSON(c, http.StatusBadRequest, "附件大小不能超过 20MB")
		return
	}
	storedName := uniqueStoredName(file.Filename)
	dir := filepath.Join(h.cfg.Storage.AttachmentDir, fmt.Sprint(ticket.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建附件目录失败")
		return
	}
	storagePath := filepath.Join(dir, storedName)
	if err := c.SaveUploadedFile(file, storagePath); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	user := currentUser(c)
	attachment := model.TicketAttachment{
		TicketID:     ticket.ID,
		UploaderID:   user.ID,
		OriginalName: filepath.Base(file.Filename),
		StoredName:   storedName,
		StoragePath:  storagePath,
		Size:         file.Size,
		ContentType:  file.Header.Get("Content-Type"),
	}
	if err := h.db.Create(&attachment).Error; err != nil {
		_ = os.Remove(storagePath)
		errorJSON(c, http.StatusBadRequest, "保存附件元数据失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "attach", ticket.Status, ticket.Status, attachment.OriginalName)
	h.db.Preload("Uploader").First(&attachment, attachment.ID)
	c.JSON(http.StatusCreated, attachment)
}

func (h *Handler) DownloadTicketAttachment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效附件 ID")
		return
	}
	var attachment model.TicketAttachment
	if err := h.db.Where("ticket_id = ? AND id = ?", ticket.ID, attachmentID).First(&attachment).Error; err != nil {
		statusForDBError(c, err, "附件不存在")
		return
	}
	c.FileAttachment(attachment.StoragePath, attachment.OriginalName)
}

func (h *Handler) applyTodoFilter(db *gorm.DB, user model.User) *gorm.DB {
	switch user.Role {
	case model.RoleAdmin:
		return db.Where("status IN ?", []model.TicketStatus{
			model.TicketStatusPendingApproval,
			model.TicketStatusApproved,
			model.TicketStatusInProgress,
			model.TicketStatusPendingAcceptance,
			model.TicketStatusRejected,
		})
	case model.RoleApprover:
		return db.Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.ticket_id = tickets.id AND ticket_workflow_steps.id = tickets.current_workflow_step_id").
			Joins("JOIN ticket_workflow_step_approvers ON ticket_workflow_step_approvers.step_id = ticket_workflow_steps.id").
			Where("tickets.status = ? AND ticket_workflow_step_approvers.user_id IN ?", model.TicketStatusPendingApproval, h.approverIDsWithProxy(user.ID))
	case model.RoleAssetManager:
		return db.Where("status IN ?", []model.TicketStatus{model.TicketStatusApproved, model.TicketStatusInProgress})
	default:
		return db.Where("status IN ? AND applicant_id = ?", []model.TicketStatus{model.TicketStatusPendingAcceptance, model.TicketStatusRejected}, user.ID)
	}
}

// approverIDsWithProxy 返回用户的审批范围：自己 + 所有设置了本人为代理审批人的用户。
// 用于待办筛选与审批权限判断（单级代理，不支持代理链）。
func (h *Handler) approverIDsWithProxy(userID uint) []uint {
	var proxied []uint
	_ = h.db.Model(&model.User{}).Where("proxy_user_id = ?", userID).Pluck("id", &proxied).Error
	ids := make([]uint, 0, len(proxied)+1)
	ids = append(ids, userID)
	ids = append(ids, proxied...)
	return ids
}

func (h *Handler) defaultApproverID(c *gin.Context, ticketType model.TicketType) (uint, bool) {
	var approverConfig model.TicketTypeApprover
	if err := h.db.Where("type = ?", ticketType).First(&approverConfig).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorJSON(c, http.StatusBadRequest, "该工单类型尚未配置默认审批人")
			return 0, false
		}
		errorJSON(c, http.StatusInternalServerError, "查询默认审批人失败")
		return 0, false
	}
	return approverConfig.ApproverID, true
}

func (h *Handler) collaborationTicket(c *gin.Context) (model.Ticket, bool) {
	id, ok := parseID(c)
	if !ok {
		return model.Ticket{}, false
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return model.Ticket{}, false
	}
	if !h.canCollaborate(c, ticket) {
		return model.Ticket{}, false
	}
	return ticket, true
}

func (h *Handler) canCollaborate(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ApproverID != nil && *ticket.ApproverID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	if user.Role == model.RoleAssetManager {
		return ticket.Status == model.TicketStatusApproved ||
			ticket.Status == model.TicketStatusInProgress ||
			ticket.Status == model.TicketStatusPendingAcceptance
	}
	errorJSON(c, http.StatusForbidden, "没有权限访问该工单协作内容")
	return false
}

func (h *Handler) canDownloadArchive(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	var count int64
	_ = h.db.Model(&model.TicketWorkflowStepApprover{}).
		Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.id = ticket_workflow_step_approvers.step_id").
		Where("ticket_workflow_steps.ticket_id = ? AND ticket_workflow_step_approvers.user_id = ?", ticket.ID, user.ID).
		Count(&count).Error
	if count > 0 {
		return true
	}
	errorJSON(c, http.StatusForbidden, "没有权限下载该工单归档")
	return false
}

func applyTicketRequest(ticket *model.Ticket, req ticketRequest) {
	ticket.Type = req.Type
	ticket.Title = req.Title
	ticket.AssetID = req.AssetID
	ticket.Priority = defaultPriority(req.Priority)
	ticket.Description = req.Description
	ticket.DeviceType = req.DeviceType
	ticket.DeviceName = req.DeviceName
	ticket.IPAddress = req.IPAddress
	ticket.OpenPorts = req.OpenPorts
	ticket.RunningServices = req.RunningServices
	ticket.AppVersion = req.AppVersion
	ticket.Manufacturer = req.Manufacturer
	ticket.Antivirus = req.Antivirus
	ticket.ChangeContent = req.ChangeContent
	ticket.Impact = req.Impact
	ticket.Remark = req.Remark
}

func (h *Handler) closeTicketWithArchive(c *gin.Context, ticket *model.Ticket) (string, string, bool) {
	var full model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Assets.Asset").Preload("Executor").Preload("Records.Actor").
		Preload("WorkflowSteps.Actor").Preload("WorkflowSteps", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc") }).
		First(&full, ticket.ID).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "加载归档数据失败")
		return "", "", false
	}
	full.Status = ticket.Status
	full.Result = ticket.Result
	full.AcceptanceResult = ticket.AcceptanceResult
	archiveNo, archivePath, err := h.archiver.Generate(c.Request.Context(), service.TicketArchiveData{Ticket: full}, h.cfg.Storage.TicketTemplatePath, h.cfg.Storage.TicketArchiveDir, h.cfg.Storage.LibreOfficeBin)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "生成归档 PDF 失败: "+err.Error())
		return "", "", false
	}
	if err := h.writeBackTicketAsset(ticket); err != nil {
		_ = os.Remove(archivePath)
		errorJSON(c, http.StatusBadRequest, "写回资产台账失败: "+err.Error())
		return "", "", false
	}
	return archiveNo, archivePath, true
}

func (h *Handler) writeBackTicketAsset(ticket *model.Ticket) error {
	// 多资产：优先取关联表；空则回退到单资产 AssetID（兼容旧工单）
	assetIDs := []uint{}
	var links []model.TicketAsset
	if err := h.db.Where("ticket_id = ?", ticket.ID).Find(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		assetIDs = append(assetIDs, link.AssetID)
	}
	if len(assetIDs) == 0 && ticket.AssetID != nil {
		assetIDs = []uint{*ticket.AssetID}
	}
	if len(assetIDs) == 0 {
		return nil
	}
	for _, assetID := range assetIDs {
		var asset model.Asset
		if err := h.db.First(&asset, assetID).Error; err != nil {
			return err
		}
		if ticket.DeviceType != "" {
			asset.AssetType = ticket.DeviceType
		}
		if ticket.DeviceName != "" {
			asset.Hostname = ticket.DeviceName
		}
		if ticket.IPAddress != "" {
			asset.IP = ticket.IPAddress
		}
		if ticket.OpenPorts != "" {
			asset.OpenPorts = ticket.OpenPorts
		}
		if ticket.RunningServices != "" {
			asset.RunningServices = ticket.RunningServices
		}
		if ticket.AppVersion != "" {
			asset.AppVersion = ticket.AppVersion
		}
		if ticket.Manufacturer != "" {
			asset.Manufacturer = ticket.Manufacturer
		}
		if err := h.db.Save(&asset).Error; err != nil {
			return err
		}
		actorID := uint(0)
		if err := (&service.AssetSnapshotter{DB: h.db}).CreateSnapshot(&asset, model.SnapshotSourceTicket, &actorID, "update"); err != nil {
			return err
		}
	}
	return nil
}

func uniqueStoredName(original string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(original))
	}
	return fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), hex.EncodeToString(random[:]), filepath.Base(original))
}

// derefStr 解引用字符串指针，空指针返回空串。
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, archiveName string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	writer, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
}

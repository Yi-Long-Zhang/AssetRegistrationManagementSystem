package httpapi

import (
	"archive/zip"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
)

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

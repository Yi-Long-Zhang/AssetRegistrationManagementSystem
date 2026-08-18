package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

// @Summary 工单列表
// @Tags tickets
// @Produce json
// @Security BearerAuth
// @Param view query string false "视图 todo/submitted/all"
// @Success 200 {object} map[string]interface{}
// @Router /tickets [get]
func swaggerListTickets() {}

// @Summary 创建工单
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ticketRequest true "工单信息"
// @Success 201 {object} model.Ticket
// @Failure 400 {object} errorResponse
// @Router /tickets [post]
func swaggerCreateTicket() {}

// @Summary 工单详情
// @Tags tickets
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} model.Ticket
// @Failure 404 {object} errorResponse
// @Router /tickets/{id} [get]
func swaggerGetTicket() {}

// @Summary 更新工单
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketRequest true "工单信息"
// @Success 200 {object} model.Ticket
// @Failure 400 {object} errorResponse
// @Router /tickets/{id} [put]
func swaggerUpdateTicket() {}

// @Summary 提交工单
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "处理意见"
// @Success 200 {object} model.Ticket
// @Failure 400 {object} errorResponse
// @Router /tickets/{id}/submit [post]
func swaggerSubmitTicket() {}

// @Summary 审批通过
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "处理意见"
// @Success 200 {object} model.Ticket
// @Failure 403 {object} errorResponse
// @Router /tickets/{id}/approve [post]
func swaggerApproveTicket() {}

// @Summary 审批驳回
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "处理意见"
// @Success 200 {object} model.Ticket
// @Failure 403 {object} errorResponse
// @Router /tickets/{id}/reject [post]
func swaggerRejectTicket() {}

// @Summary 开始执行
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "处理意见"
// @Success 200 {object} model.Ticket
// @Router /tickets/{id}/start [post]
func swaggerStartTicket() {}

// @Summary 执行完成
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "执行结果"
// @Success 200 {object} model.Ticket
// @Router /tickets/{id}/complete [post]
func swaggerCompleteTicket() {}

// @Summary 验收通过并归档
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "验收结果"
// @Success 200 {object} model.Ticket
// @Router /tickets/{id}/accept [post]
func swaggerAcceptTicket() {}

// @Summary 取消工单
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketActionRequest false "取消原因"
// @Success 200 {object} model.Ticket
// @Router /tickets/{id}/cancel [post]
func swaggerCancelTicket() {}

// @Summary 工单评论列表
// @Tags tickets
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} map[string]interface{}
// @Router /tickets/{id}/comments [get]
func swaggerListTicketComments() {}

// @Summary 创建工单评论
// @Tags tickets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param body body ticketCommentRequest true "评论"
// @Success 201 {object} model.TicketComment
// @Failure 400 {object} errorResponse
// @Router /tickets/{id}/comments [post]
func swaggerCreateTicketComment() {}

// @Summary 工单附件列表
// @Tags tickets
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {object} map[string]interface{}
// @Router /tickets/{id}/attachments [get]
func swaggerListTicketAttachments() {}

// @Summary 上传工单附件
// @Tags tickets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param file formData file true "附件"
// @Success 201 {object} model.TicketAttachment
// @Failure 400 {object} errorResponse
// @Router /tickets/{id}/attachments [post]
func swaggerUploadTicketAttachment() {}

// @Summary 下载工单附件
// @Tags tickets
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Param attachmentId path int true "附件 ID"
// @Success 200 {file} file
// @Router /tickets/{id}/attachments/{attachmentId}/download [get]
func swaggerDownloadTicketAttachment() {}

// @Summary 下载工单归档 PDF
// @Tags tickets
// @Produce application/pdf
// @Security BearerAuth
// @Param id path int true "工单 ID"
// @Success 200 {file} file
// @Failure 400 {object} errorResponse
// @Router /tickets/{id}/archive/download [get]
func swaggerDownloadTicketArchive() {}

// @Summary 批量下载工单归档 ZIP
// @Tags tickets
// @Accept json
// @Produce application/zip
// @Security BearerAuth
// @Param body body ticketArchiveBatchRequest true "工单 ID 列表"
// @Success 200 {file} file
// @Failure 400 {object} errorResponse
// @Router /tickets/archives/download [post]
func swaggerDownloadTicketArchives() {}

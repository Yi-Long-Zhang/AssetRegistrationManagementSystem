package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

// @Summary 用户登录
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "登录参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /auth/login [post]
func swaggerLogin() {}

// @Summary 用户退出
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /auth/logout [post]
func swaggerLogout() {}

// @Summary 当前用户
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} errorResponse
// @Router /auth/me [get]
func swaggerMe() {}

// @Summary 角色列表
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} errorResponse
// @Router /roles [get]
func swaggerListRoles() {}

// @Summary 用户列表
// @Tags users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /users [get]
func swaggerListUsers() {}

// @Summary 创建用户
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body userRequest true "用户信息"
// @Success 201 {object} model.User
// @Failure 400 {object} errorResponse
// @Router /users [post]
func swaggerCreateUser() {}

// @Summary 更新用户
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户 ID"
// @Param body body userRequest true "用户信息"
// @Success 200 {object} model.User
// @Failure 400 {object} errorResponse
// @Router /users/{id} [put]
func swaggerUpdateUser() {}

// @Summary AD 配置详情
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /ad/config [get]
func swaggerGetADConfig() {}

// @Summary 保存 AD 配置
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body adConfigRequest true "AD 配置"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /ad/config [put]
func swaggerSaveADConfig() {}

// @Summary 测试 AD 连接
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /ad/test [post]
func swaggerTestADConnection() {}

// @Summary 查询 AD 用户
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body adLookupRequest true "查询参数"
// @Success 200 {object} service.ADUserInfo
// @Failure 400 {object} errorResponse
// @Router /ad/lookup-user [post]
func swaggerLookupADUser() {}

// @Summary 导入 AD 用户
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body adImportRequest true "导入参数"
// @Success 200 {object} model.User
// @Failure 400 {object} errorResponse
// @Router /ad/import-user [post]
func swaggerImportADUser() {}

// @Summary 邮件配置详情
// @Tags settings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /settings/mail [get]
func swaggerGetMailConfig() {}

// @Summary 保存邮件配置
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body mailConfigRequest true "邮件配置"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /settings/mail [put]
func swaggerSaveMailConfig() {}

// @Summary 测试邮件发送
// @Tags settings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body mailTestRequest true "测试参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /settings/mail/test [post]
func swaggerTestMailConfig() {}

// @Summary 资产列表
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param q query string false "关键词"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} map[string]interface{}
// @Router /assets [get]
func swaggerListAssets() {}

// @Summary 创建资产
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body assetRequest true "资产信息"
// @Success 201 {object} model.Asset
// @Failure 400 {object} errorResponse
// @Router /assets [post]
func swaggerCreateAsset() {}

// @Summary 资产统计
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /assets/stats [get]
func swaggerAssetStats() {}

// @Summary 资产详情
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path int true "资产 ID"
// @Success 200 {object} model.Asset
// @Failure 404 {object} errorResponse
// @Router /assets/{id} [get]
func swaggerGetAsset() {}

// @Summary 更新资产
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "资产 ID"
// @Param body body assetRequest true "资产信息"
// @Success 200 {object} model.Asset
// @Failure 400 {object} errorResponse
// @Router /assets/{id} [put]
func swaggerUpdateAsset() {}

// @Summary 删除资产
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path int true "资产 ID"
// @Success 204
// @Failure 400 {object} errorResponse
// @Router /assets/{id} [delete]
func swaggerDeleteAsset() {}

// @Summary 下载资产导入模板
// @Tags assets
// @Produce application/octet-stream
// @Security BearerAuth
// @Param format query string false "模板格式 csv 或 xlsx"
// @Success 200 {file} file
// @Router /assets/template [get]
func swaggerDownloadAssetImportTemplate() {}

// @Summary 导入资产
// @Tags assets
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "CSV 或 XLSX 文件"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /assets/import [post]
func swaggerImportAssets() {}

// @Summary 导出资产 CSV
// @Tags assets
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {file} file
// @Router /assets/export [get]
func swaggerExportAssets() {}

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

// @Summary 流程配置列表
// @Tags workflows
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /workflows [get]
func swaggerListWorkflows() {}

// @Summary 工单类型流程配置
// @Tags workflows
// @Produce json
// @Security BearerAuth
// @Param type path string true "工单类型"
// @Success 200 {object} model.TicketWorkflow
// @Failure 404 {object} errorResponse
// @Router /workflows/{type} [get]
func swaggerGetWorkflow() {}

// @Summary 保存工单类型流程配置
// @Tags workflows
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param type path string true "工单类型"
// @Param body body workflowRequest true "流程配置"
// @Success 200 {object} model.TicketWorkflow
// @Failure 400 {object} errorResponse
// @Router /workflows/{type} [put]
func swaggerSaveWorkflow() {}

// @Summary 启用工单类型流程
// @Tags workflows
// @Produce json
// @Security BearerAuth
// @Param type path string true "工单类型"
// @Success 200 {object} model.TicketWorkflow
// @Failure 400 {object} errorResponse
// @Router /workflows/{type}/enable [post]
func swaggerEnableWorkflow() {}

// @Summary 发现规则列表
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.DiscoveryRule
// @Failure 401 {object} errorResponse
// @Router /discovery/rules [get]
func swaggerListDiscoveryRules() {}

// @Summary 创建发现规则
// @Tags discovery
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body discoveryRuleRequest true "发现规则"
// @Success 201 {object} model.DiscoveryRule
// @Failure 400 {object} errorResponse
// @Router /discovery/rules [post]
func swaggerCreateDiscoveryRule() {}

// @Summary 更新发现规则
// @Tags discovery
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Param body body discoveryRuleRequest true "发现规则"
// @Success 200 {object} model.DiscoveryRule
// @Failure 400 {object} errorResponse
// @Router /discovery/rules/{id} [put]
func swaggerUpdateDiscoveryRule() {}

// @Summary 删除发现规则
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /discovery/rules/{id} [delete]
func swaggerDeleteDiscoveryRule() {}

// @Summary 手动触发发现任务
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Success 202 {object} model.DiscoveryRun
// @Failure 400 {object} errorResponse
// @Router /discovery/rules/{id}/run [post]
func swaggerStartDiscoveryRun() {}

// @Summary 试跑发现规则
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Param id path int true "规则 ID"
// @Success 200 {object} map[string]interface{}
// @Router /discovery/rules/{id}/test [post]
func swaggerTestDiscoveryRun() {}

// @Summary 运行记录列表
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Param ruleId query int false "规则 ID"
// @Param status query string false "状态"
// @Success 200 {object} map[string]interface{}
// @Router /discovery/runs [get]
func swaggerListDiscoveryRuns() {}

// @Summary 运行记录详情
// @Tags discovery
// @Produce json
// @Security BearerAuth
// @Param id path int true "运行 ID"
// @Success 200 {object} model.DiscoveryRun
// @Failure 400 {object} errorResponse
// @Router /discovery/runs/{id} [get]
func swaggerGetDiscoveryRun() {}

// @Summary 纳管新发现主机
// @Tags discovery
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "运行 ID"
// @Param body body discoveryHostActionRequest true "主机 ID 列表"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /discovery/runs/{id}/adopt [post]
func swaggerAdoptDiscoveryHosts() {}

// @Summary 应用发现变更到资产
// @Tags discovery
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "运行 ID"
// @Param body body discoveryHostActionRequest true "主机 ID 列表"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /discovery/runs/{id}/apply [post]
func swaggerApplyDiscoveryHosts() {}

// @Summary 资产变更历史
// @Tags assets
// @Produce json
// @Security BearerAuth
// @Param id path int true "资产 ID"
// @Success 200 {array} model.AssetSnapshot
// @Failure 400 {object} errorResponse
// @Router /assets/{id}/history [get]
func swaggerListAssetHistory() {}

// @Summary 批量删除资产
// @Tags assets
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body batchDeleteRequest true "资产 ID 列表"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /assets/batch-delete [post]
func swaggerBatchDeleteAssets() {}

// @Summary 软件许可证列表
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Router /licenses [get]
func swaggerListSoftwareLicenses() {}

// @Summary 创建软件许可证
// @Tags software-licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body softwareLicenseRequest true "软件许可证"
// @Success 201 {object} model.SoftwareLicense
// @Failure 400 {object} errorResponse
// @Router /licenses [post]
func swaggerCreateSoftwareLicense() {}

// @Summary 更新软件许可证
// @Tags software-licenses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param body body softwareLicenseRequest true "软件许可证"
// @Success 200 {object} model.SoftwareLicense
// @Failure 400 {object} errorResponse
// @Router /licenses/{id} [put]
func swaggerUpdateSoftwareLicense() {}

// @Summary 删除软件许可证
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id} [delete]
func swaggerDeleteSoftwareLicense() {}

// @Summary 查看许可证密钥明文
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/reveal [post]
func swaggerRevealSoftwareLicense() {}

// @Summary 下载许可证导入模板
// @Tags software-licenses
// @Produce application/octet-stream
// @Security BearerAuth
// @Param format query string false "模板格式 csv 或 xlsx"
// @Success 200 {file} file
// @Router /licenses/template [get]
func swaggerDownloadLicenseImportTemplate() {}

// @Summary 导入软件许可证
// @Tags software-licenses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "CSV 或 XLSX 文件"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/import [post]
func swaggerImportLicenses() {}

// @Summary 导出软件许可证 CSV
// @Tags software-licenses
// @Produce text/csv
// @Security BearerAuth
// @Success 200 {file} file
// @Router /licenses/export [get]
func swaggerExportLicenses() {}

// @Summary 许可证附件列表
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Success 200 {object} map[string]interface{}
// @Router /licenses/{id}/attachments [get]
func swaggerListLicenseAttachments() {}

// @Summary 上传许可证附件
// @Tags software-licenses
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param file formData file true "附件"
// @Success 201 {object} model.LicenseAttachment
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/attachments [post]
func swaggerUploadLicenseAttachment() {}

// @Summary 下载许可证附件
// @Tags software-licenses
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param attachmentId path int true "附件 ID"
// @Success 200 {file} file
// @Router /licenses/{id}/attachments/{attachmentId}/download [get]
func swaggerDownloadLicenseAttachment() {}

// @Summary 删除许可证附件
// @Tags software-licenses
// @Produce json
// @Security BearerAuth
// @Param id path int true "许可证 ID"
// @Param attachmentId path int true "附件 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /licenses/{id}/attachments/{attachmentId} [delete]
func swaggerDeleteLicenseAttachment() {}

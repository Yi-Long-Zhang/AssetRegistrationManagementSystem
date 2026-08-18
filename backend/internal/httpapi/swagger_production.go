package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.BackgroundTask
	_ service.BackupInfo
)

type changePasswordDocRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type reauthenticateDocRequest struct {
	Password string `json:"password"`
}

// @Summary 修改当前用户密码
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body changePasswordDocRequest true "密码修改参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /auth/change-password [post]
func swaggerChangePassword() {}

// @Summary 二次认证
// @Description 刷新当前会话的敏感操作认证有效期
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body reauthenticateDocRequest true "当前密码"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} errorResponse
// @Router /auth/reauth [post]
func swaggerReauthenticate() {}

// @Summary 当前用户会话列表
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} errorResponse
// @Router /auth/sessions [get]
func swaggerListSessions() {}

// @Summary 撤销指定会话
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Param id path string true "会话 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Router /auth/sessions/{id} [delete]
func swaggerRevokeSession() {}

// @Summary 全端退出
// @Description 撤销当前用户的全部服务端会话
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} errorResponse
// @Router /auth/sessions/revoke-all [post]
func swaggerRevokeAllSessions() {}

// @Summary 校验完整备份
// @Tags backups
// @Produce json
// @Security BearerAuth
// @Param name path string true "备份文件名"
// @Success 200 {object} service.BackupInfo
// @Failure 400 {object} errorResponse
// @Router /backups/{name}/verify [post]
func swaggerVerifyBackup() {}

// @Summary 后台任务列表
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param kind query string false "任务类型"
// @Param status query string false "任务状态"
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} errorResponse
// @Router /tasks [get]
func swaggerListBackgroundTasks() {}

// @Summary 后台任务详情
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Success 200 {object} model.BackgroundTask
// @Failure 404 {object} errorResponse
// @Router /tasks/{id} [get]
func swaggerGetBackgroundTask() {}

// @Summary 重试后台任务
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Success 200 {object} model.BackgroundTask
// @Failure 400 {object} errorResponse
// @Router /tasks/{id}/retry [post]
func swaggerRetryBackgroundTask() {}

// @Summary 确认后台任务告警
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param id path int true "任务 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} errorResponse
// @Router /tasks/{id}/acknowledge [post]
func swaggerAcknowledgeBackgroundTask() {}

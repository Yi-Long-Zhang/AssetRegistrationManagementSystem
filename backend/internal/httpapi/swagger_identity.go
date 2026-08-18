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

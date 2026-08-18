package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

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

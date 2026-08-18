package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

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

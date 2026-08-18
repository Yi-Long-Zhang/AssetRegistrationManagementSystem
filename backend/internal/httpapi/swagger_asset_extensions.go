package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

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

package httpapi

import (
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"
)

var (
	_ model.User
	_ service.ADUserInfo
)

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

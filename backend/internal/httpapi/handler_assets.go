package httpapi

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) ListAssets(c *gin.Context) {
	var assets []model.Asset
	page, pageSize := assetPagination(c)
	db := scopeAssetsByRole(applyAssetFilters(h.db.Model(&model.Asset{}), c), currentUser(c))
	var total int64
	if err := db.Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产失败")
		return
	}
	db = applyAssetSort(db, c).Offset((page - 1) * pageSize).Limit(pageSize)
	if err := db.Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": assets, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) AssetStats(c *gin.Context) {
	user := currentUser(c)
	scoped := func(db *gorm.DB) *gorm.DB {
		return scopeAssetsByRole(db, user)
	}
	base := scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c))
	var total int64
	if err := base.Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产统计失败")
		return
	}
	var assets []model.Asset
	if err := scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c)).Select("open_ports", "running_services", "online_status").Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产统计失败")
		return
	}
	openPortValues := make([]string, 0, len(assets))
	serviceValues := make([]string, 0, len(assets))
	openPortAssetCount := int64(0)
	onlineCounts := map[string]int64{}
	for _, asset := range assets {
		status := string(asset.OnlineStatus)
		if status == "" {
			status = string(model.AssetOnlineStatusUnknown)
		}
		onlineCounts[status]++
		if strings.TrimSpace(asset.OpenPorts) != "" {
			openPortAssetCount++
			openPortValues = append(openPortValues, asset.OpenPorts)
		}
		if strings.TrimSpace(asset.RunningServices) != "" {
			serviceValues = append(serviceValues, asset.RunningServices)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total":              total,
		"subnetCount":        len(assetGroupedCounts(scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c)), "subnet", 1000000)),
		"openPortAssetCount": openPortAssetCount,
		"byOnlineStatus":     onlineCounts,
		"byAssetType":        assetGroupedCounts(scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c)), "asset_type", 10),
		"bySubnet":           assetGroupedCounts(scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c)), "subnet", 10),
		"byOwner":            assetGroupedCounts(scoped(applyAssetFilters(h.db.Model(&model.Asset{}), c)), "owner", 10),
		"topOpenPorts":       topAssetTokens(openPortValues, 10, true),
		"topServices":        topAssetTokens(serviceValues, 10, false),
	})
}

func (h *Handler) CreateAsset(c *gin.Context) {
	var req assetRequest
	if !bind(c, &req) {
		return
	}
	asset := req.toModel()
	if err := h.db.Create(&asset).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建资产失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "asset", asset.ID, "create", asset.AssetNo)
	h.checkIPConflict(currentUser(c).ID, &asset)
	c.JSON(http.StatusCreated, asset)
}

func (h *Handler) GetAsset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var asset model.Asset
	if !h.findByID(c, id, &asset) {
		return
	}
	if !canViewAsset(currentUser(c), asset) {
		errorJSON(c, http.StatusForbidden, "无权查看该资产")
		return
	}
	c.JSON(http.StatusOK, asset)
}

func (h *Handler) UpdateAsset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var asset model.Asset
	if !h.findByID(c, id, &asset) {
		return
	}
	var req assetRequest
	if !bind(c, &req) {
		return
	}
	updated := req.toModel()
	updated.ID = asset.ID
	updated.CreatedAt = asset.CreatedAt
	if err := h.db.Save(&updated).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新资产失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "asset", updated.ID, "update", updated.AssetNo)
	h.checkIPConflict(currentUser(c).ID, &updated)
	actorID := currentUser(c).ID
	if err := (&service.AssetSnapshotter{DB: h.db}).CreateSnapshot(&updated, model.SnapshotSourceManual, &actorID, "update"); err != nil {
		log.Printf("asset snapshot failed for asset %d: %v", updated.ID, err)
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteAsset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.db.Delete(&model.Asset{}, id).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "删除资产失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "asset", id, "delete", "")
	c.Status(http.StatusNoContent)
}

// RetireAsset 资产退役归档：状态置为 retired，写快照与审计（保留历史数据，不删除）。
// @Summary 资产退役归档
// @Description 将资产状态置为退役（retired），保留快照与审计记录
// @Tags assets
// @Produce json
// @Param id path int true "资产 ID"
// @Success 200 {object} model.Asset
// @Failure 400 {object} map[string]string
// @Router /assets/{id}/retire [post]
// @Security BearerAuth
func (h *Handler) RetireAsset(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var asset model.Asset
	if err := h.db.First(&asset, id).Error; err != nil {
		statusForDBError(c, err, "资产不存在")
		return
	}
	if asset.Status == model.AssetStatusRetired {
		errorJSON(c, http.StatusBadRequest, "该资产已处于退役状态")
		return
	}
	asset.Status = model.AssetStatusRetired
	if err := h.db.Save(&asset).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "退役资产失败: "+err.Error())
		return
	}
	snapper := service.AssetSnapshotter{DB: h.db}
	if err := snapper.CreateSnapshot(&asset, model.SnapshotSourceManual, nil, "retire"); err != nil {
		errorJSON(c, http.StatusBadRequest, "生成快照失败: "+err.Error())
		return
	}
	actor := currentUser(c).ID
	h.audit(actor, "asset", asset.ID, "retire", "资产退役归档: "+asset.AssetNo)
	c.JSON(http.StatusOK, asset)
}

type batchDeleteRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

// BatchDeleteAssets 批量删除资产：清理关联快照、解除发现结果引用，事务内硬删除。
func (h *Handler) BatchDeleteAssets(c *gin.Context) {
	var req batchDeleteRequest
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		errorJSON(c, http.StatusBadRequest, "未选择要删除的资产")
		return
	}
	if len(req.IDs) > 200 {
		errorJSON(c, http.StatusBadRequest, "单次最多删除 200 台资产")
		return
	}
	var count int64
	h.db.Model(&model.Asset{}).Where("id IN ?", req.IDs).Count(&count)
	if count == 0 {
		errorJSON(c, http.StatusBadRequest, "未找到要删除的资产")
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// 清理关联快照
		if err := tx.Where("asset_id IN ?", req.IDs).Delete(&model.AssetSnapshot{}).Error; err != nil {
			return err
		}
		// 解除发现结果对资产的引用，避免孤儿引用
		if err := tx.Model(&model.DiscoveredHost{}).Where("matched_asset_id IN ?", req.IDs).
			Updates(map[string]any{"matched_asset_id": nil, "change_type": model.DiscoveryChangeNew}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&model.Asset{}, req.IDs).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "批量删除失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "asset", 0, "batch_delete", fmt.Sprintf("批量删除 %d 台资产", count))
	c.JSON(http.StatusOK, gin.H{"deleted": count})
}

type batchAssetUpdateRequest struct {
	IDs    []uint         `json:"ids" binding:"required"`
	Fields map[string]any `json:"fields" binding:"required"`
}

// batchAssetUpdateColumns 批量编辑字段白名单：json 字段名 → 数据库列名。
// GORM Updates(map) 使用数据库列名，故 camelCase 字段必须显式映射。
var batchAssetUpdateColumns = map[string]string{
	"owner":              "owner",
	"department":         "department",
	"location":           "location",
	"rack":               "rack",
	"rackPosition":       "rack_position",
	"environment":        "environment",
	"businessSystem":     "business_system",
	"maintenanceVendor":  "maintenance_vendor",
	"warrantyExpireDate": "warranty_expire_date",
	"status":             "status",
	"remark":             "remark",
}

func validAssetStatus(status string) bool {
	switch model.AssetStatus(status) {
	case model.AssetStatusPending, model.AssetStatusInUse, model.AssetStatusMaintenance,
		model.AssetStatusRetired, model.AssetStatusDecommission:
		return true
	}
	return false
}

// BatchUpdateAssets 批量编辑资产：仅允许白名单字段，逐台更新并生成快照与审计。
func (h *Handler) BatchUpdateAssets(c *gin.Context) {
	var req batchAssetUpdateRequest
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		errorJSON(c, http.StatusBadRequest, "未选择要修改的资产")
		return
	}
	if len(req.IDs) > 200 {
		errorJSON(c, http.StatusBadRequest, "单次最多修改 200 台资产")
		return
	}
	if len(req.Fields) == 0 {
		errorJSON(c, http.StatusBadRequest, "未提供要修改的字段")
		return
	}
	updates := map[string]any{}
	for key, value := range req.Fields {
		column, ok := batchAssetUpdateColumns[key]
		if !ok {
			errorJSON(c, http.StatusBadRequest, "字段不允许批量修改: "+key)
			return
		}
		updates[column] = value
	}
	// 日期字段：字符串 → time.Time；空字符串表示置空；非字符串类型直接拒绝
	if value, ok := updates["warranty_expire_date"]; ok {
		str, isStr := value.(string)
		if !isStr {
			errorJSON(c, http.StatusBadRequest, "维保到期日期必须是 YYYY-MM-DD 字符串")
			return
		}
		if strings.TrimSpace(str) == "" {
			updates["warranty_expire_date"] = nil
		} else {
			parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(str), time.Local)
			if err != nil {
				errorJSON(c, http.StatusBadRequest, "维保到期日期格式应为 YYYY-MM-DD")
				return
			}
			updates["warranty_expire_date"] = parsed
		}
	}
	if statusValue, ok := updates["status"]; ok {
		statusStr, _ := statusValue.(string)
		if !validAssetStatus(statusStr) {
			errorJSON(c, http.StatusBadRequest, "资产状态不合法")
			return
		}
	}
	actorID := currentUser(c).ID
	updatedCount := 0
	err := h.db.Transaction(func(tx *gorm.DB) error {
		var assets []model.Asset
		if err := tx.Where("id IN ?", req.IDs).Find(&assets).Error; err != nil {
			return err
		}
		if len(assets) == 0 {
			return errors.New("未找到要修改的资产")
		}
		snapper := service.AssetSnapshotter{DB: tx}
		for i := range assets {
			asset := assets[i]
			if err := tx.Model(&asset).Updates(updates).Error; err != nil {
				return err
			}
			var updated model.Asset
			if err := tx.First(&updated, asset.ID).Error; err != nil {
				return err
			}
			if err := snapper.CreateSnapshot(&updated, model.SnapshotSourceManual, &actorID, "batch_update"); err != nil {
				return err
			}
			if err := tx.Create(&model.AuditLog{
				ActorID:  actorID,
				Entity:   "asset",
				EntityID: updated.ID,
				Action:   "batch_update",
				Detail:   "批量编辑: " + updated.AssetNo,
			}).Error; err != nil {
				return err
			}
			updatedCount++
		}
		return nil
	})
	if err != nil {
		log.Printf("batch update assets: %v", err)
		errorJSON(c, http.StatusBadRequest, "批量修改失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updatedCount})
}

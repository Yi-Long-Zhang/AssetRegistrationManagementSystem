package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// createStocktakeRequest 创建盘点单请求
type createStocktakeRequest struct {
	Name      string `json:"name" binding:"required"`
	Remark    string `json:"remark"`
	AssetType string `json:"assetType"` // 可选：按资产类型过滤盘点范围，空=全部
}

// stocktakeItemRequest 盘点项核对请求
type stocktakeItemRequest struct {
	Result string `json:"result" binding:"required"` // matched / missing
	Remark string `json:"remark"`
}

// CreateStocktake 创建盘点单（按范围快照资产生成盘点明细）。
// @Summary 创建盘点单
// @Description 按范围快照资产并生成盘点明细（admin/asset_manager）
// @Tags stocktakes
// @Accept json
// @Produce json
// @Param body body createStocktakeRequest true "盘点单"
// @Success 201 {object} model.StocktakeTask
// @Security BearerAuth
// @Router /stocktakes [post]
func (h *Handler) CreateStocktake(c *gin.Context) {
	var req createStocktakeRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	task := model.StocktakeTask{Name: strings.TrimSpace(req.Name), Status: "in_progress", CreatorID: user.ID, Remark: req.Remark}

	var assetIDs []uint
	db := h.db.Model(&model.Asset{})
	if strings.TrimSpace(req.AssetType) != "" {
		db = db.Where("asset_type = ?", strings.TrimSpace(req.AssetType))
	}
	if err := db.Pluck("id", &assetIDs).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "快照资产失败")
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, id := range assetIDs {
			if err := tx.Create(&model.StocktakeItem{TaskID: task.ID, AssetID: id, Result: "pending"}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "创建盘点单失败: "+err.Error())
		return
	}
	h.db.Preload("Items").First(&task, task.ID)
	c.JSON(http.StatusCreated, task)
}

// ListStocktakes 盘点单列表。
// @Summary 盘点单列表
// @Description 查询盘点单列表（admin/asset_manager）
// @Tags stocktakes
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /stocktakes [get]
func (h *Handler) ListStocktakes(c *gin.Context) {
	var tasks []model.StocktakeTask
	if err := h.db.Preload("Creator").Order("id desc").Find(&tasks).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询盘点单失败")
		return
	}
	// 附加统计：总数/已核对/盘亏
	type item struct {
		model.StocktakeTask
		Total   int64 `json:"total"`
		Checked int64 `json:"checked"`
		Missing int64 `json:"missing"`
	}
	out := make([]item, 0, len(tasks))
	for _, t := range tasks {
		row := item{StocktakeTask: t}
		h.db.Model(&model.StocktakeItem{}).Where("task_id = ?", t.ID).Count(&row.Total)
		h.db.Model(&model.StocktakeItem{}).Where("task_id = ? AND result <> 'pending'", t.ID).Count(&row.Checked)
		h.db.Model(&model.StocktakeItem{}).Where("task_id = ? AND result = 'missing'", t.ID).Count(&row.Missing)
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// GetStocktake 盘点单详情（含明细）。
// @Summary 盘点单详情
// @Description 查询盘点单与明细（admin/asset_manager）
// @Tags stocktakes
// @Produce json
// @Param id path int true "盘点单 ID"
// @Success 200 {object} model.StocktakeTask
// @Security BearerAuth
// @Router /stocktakes/{id} [get]
func (h *Handler) GetStocktake(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var task model.StocktakeTask
	if err := h.db.Preload("Creator").Preload("Items.Asset").First(&task, id).Error; err != nil {
		statusForDBError(c, err, "盘点单不存在")
		return
	}
	c.JSON(http.StatusOK, task)
}

// UpdateStocktakeItem 核对盘点项。
// @Summary 核对盘点项
// @Description 标记盘点项为 matched/missing（admin/asset_manager）
// @Tags stocktakes
// @Accept json
// @Produce json
// @Param id path int true "盘点单 ID"
// @Param itemId path int true "盘点项 ID"
// @Param body body stocktakeItemRequest true "核对结果"
// @Success 200 {object} model.StocktakeItem
// @Security BearerAuth
// @Router /stocktakes/{id}/items/{itemId} [put]
func (h *Handler) UpdateStocktakeItem(c *gin.Context) {
	taskID, ok := parseID(c)
	if !ok {
		return
	}
	itemID, ok := parseIDPath(c, "itemId")
	if !ok {
		return
	}
	var req stocktakeItemRequest
	if !bind(c, &req) {
		return
	}
	if req.Result != "matched" && req.Result != "missing" && req.Result != "pending" {
		errorJSON(c, http.StatusBadRequest, "核对结果应为 matched/missing/pending")
		return
	}
	var item model.StocktakeItem
	if err := h.db.Where("id = ? AND task_id = ?", itemID, taskID).First(&item).Error; err != nil {
		statusForDBError(c, err, "盘点项不存在")
		return
	}
	var task model.StocktakeTask
	if err := h.db.First(&task, taskID).Error; err != nil || task.Status == "closed" {
		errorJSON(c, http.StatusBadRequest, "盘点单已关闭，不可修改")
		return
	}
	item.Result = req.Result
	item.Remark = req.Remark
	now := time.Now()
	if req.Result == "pending" {
		item.CheckedAt = nil
	} else {
		item.CheckedAt = &now
	}
	if err := h.db.Save(&item).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新盘点项失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, item)
}

// CloseStocktake 关闭盘点单。
// @Summary 关闭盘点单
// @Description 关闭盘点单并产出差异统计（admin/asset_manager）
// @Tags stocktakes
// @Produce json
// @Param id path int true "盘点单 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /stocktakes/{id}/close [post]
func (h *Handler) CloseStocktake(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var task model.StocktakeTask
	if err := h.db.First(&task, id).Error; err != nil {
		statusForDBError(c, err, "盘点单不存在")
		return
	}
	if task.Status == "closed" {
		errorJSON(c, http.StatusBadRequest, "盘点单已关闭")
		return
	}
	var total, checked, matched, missing int64
	h.db.Model(&model.StocktakeItem{}).Where("task_id = ?", id).Count(&total)
	h.db.Model(&model.StocktakeItem{}).Where("task_id = ? AND result <> 'pending'", id).Count(&checked)
	h.db.Model(&model.StocktakeItem{}).Where("task_id = ? AND result = 'matched'", id).Count(&matched)
	h.db.Model(&model.StocktakeItem{}).Where("task_id = ? AND result = 'missing'", id).Count(&missing)
	now := time.Now()
	task.Status = "closed"
	task.ClosedAt = &now
	if err := h.db.Save(&task).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "关闭盘点单失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "checked": checked, "matched": matched, "missing": missing})
}

// ExportStocktake 导出盘点差异报告 CSV。
// @Summary 导出盘点差异报告
// @Description 导出盘点明细为 CSV（含盘亏资产，admin/asset_manager）
// @Tags stocktakes
// @Produce text/csv
// @Param id path int true "盘点单 ID"
// @Success 200 {string} string "CSV 文件"
// @Security BearerAuth
// @Router /stocktakes/{id}/export [get]
func (h *Handler) ExportStocktake(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var task model.StocktakeTask
	if err := h.db.First(&task, id).Error; err != nil {
		statusForDBError(c, err, "盘点单不存在")
		return
	}
	var items []model.StocktakeItem
	if err := h.db.Preload("Asset").Where("task_id = ?", id).Order("id asc").Find(&items).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询盘点明细失败")
		return
	}
	var buf strings.Builder
	writeCSVRow := func(row ...string) {
		for i, v := range row {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(csvEscape(v))
		}
		buf.WriteString("\n")
	}
	writeCSVRow("资产盘点差异报告", task.Name, "生成时间", time.Now().Format("2006-01-02 15:04:05"))
	writeCSVRow()
	writeCSVRow("资产编号", "IP", "主机名", "资产类型", "核对结果", "备注")
	for _, item := range items {
		result := map[string]string{"pending": "未核对", "matched": "一致", "missing": "盘亏"}[item.Result]
		writeCSVRow(item.Asset.AssetNo, item.Asset.IP, item.Asset.Hostname, item.Asset.AssetType, result, item.Remark)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="stocktake-%d.csv"`, id))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", []byte("\ufeff"+buf.String()))
}

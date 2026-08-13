package httpapi

import (
	"net/http"
	"strings"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// roomRequest 机房请求体
type roomRequest struct {
	Name     string `json:"name" binding:"required"`
	Location string `json:"location"`
	Remark   string `json:"remark"`
}

// rackRequest 机柜请求体
type rackRequest struct {
	RoomID uint   `json:"roomId" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Units  int    `json:"units"`
	Remark string `json:"remark"`
}

// ListRooms 机房列表（含机柜）。
// @Summary 机房列表
// @Description 查询机房及机柜列表（admin/asset_manager）
// @Tags rooms
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /rooms [get]
func (h *Handler) ListRooms(c *gin.Context) {
	var rooms []model.DatacenterRoom
	if err := h.db.Preload("Racks", func(db *gorm.DB) *gorm.DB { return db.Order("id asc") }).Order("id asc").Find(&rooms).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询机房失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rooms})
}

// CreateRoom 新增机房。
// @Summary 新增机房
// @Description 创建机房（admin/asset_manager）
// @Tags rooms
// @Accept json
// @Produce json
// @Param body body roomRequest true "机房"
// @Success 201 {object} model.DatacenterRoom
// @Security BearerAuth
// @Router /rooms [post]
func (h *Handler) CreateRoom(c *gin.Context) {
	var req roomRequest
	if !bind(c, &req) {
		return
	}
	room := model.DatacenterRoom{Name: strings.TrimSpace(req.Name), Location: req.Location, Remark: req.Remark}
	if err := h.db.Create(&room).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建机房失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, room)
}

// UpdateRoom 更新机房。
// @Summary 更新机房
// @Description 更新机房信息（admin/asset_manager）
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path int true "机房 ID"
// @Param body body roomRequest true "机房"
// @Success 200 {object} model.DatacenterRoom
// @Security BearerAuth
// @Router /rooms/{id} [put]
func (h *Handler) UpdateRoom(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req roomRequest
	if !bind(c, &req) {
		return
	}
	var room model.DatacenterRoom
	if err := h.db.First(&room, id).Error; err != nil {
		statusForDBError(c, err, "机房不存在")
		return
	}
	room.Name = strings.TrimSpace(req.Name)
	room.Location = req.Location
	room.Remark = req.Remark
	if err := h.db.Save(&room).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新机房失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, room)
}

// DeleteRoom 删除机房（连带删除其机柜）。
// @Summary 删除机房
// @Description 删除机房及其机柜（admin/asset_manager）
// @Tags rooms
// @Param id path int true "机房 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /rooms/{id} [delete]
func (h *Handler) DeleteRoom(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", id).Delete(&model.Rack{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.DatacenterRoom{}, id).Error
	})
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除机房失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListRacks 机柜列表（可按机房过滤）。
// @Summary 机柜列表
// @Description 查询机柜列表（可按 roomId 过滤，admin/asset_manager）
// @Tags racks
// @Produce json
// @Param roomId query int false "机房 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /racks [get]
func (h *Handler) ListRacks(c *gin.Context) {
	db := h.db.Preload("Room").Order("id asc")
	if roomID := c.Query("roomId"); roomID != "" {
		db = db.Where("room_id = ?", roomID)
	}
	var racks []model.Rack
	if err := db.Find(&racks).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询机柜失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": racks})
}

// CreateRack 新增机柜。
// @Summary 新增机柜
// @Description 创建机柜（admin/asset_manager）
// @Tags racks
// @Accept json
// @Produce json
// @Param body body rackRequest true "机柜"
// @Success 201 {object} model.Rack
// @Security BearerAuth
// @Router /racks [post]
func (h *Handler) CreateRack(c *gin.Context) {
	var req rackRequest
	if !bind(c, &req) {
		return
	}
	if !h.roomExists(req.RoomID) {
		errorJSON(c, http.StatusBadRequest, "机房不存在")
		return
	}
	units := req.Units
	if units <= 0 {
		units = 42
	}
	rack := model.Rack{RoomID: req.RoomID, Name: strings.TrimSpace(req.Name), Units: units, Remark: req.Remark}
	if err := h.db.Create(&rack).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建机柜失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, rack)
}

// UpdateRack 更新机柜。
// @Summary 更新机柜
// @Description 更新机柜信息（admin/asset_manager）
// @Tags racks
// @Accept json
// @Produce json
// @Param id path int true "机柜 ID"
// @Param body body rackRequest true "机柜"
// @Success 200 {object} model.Rack
// @Security BearerAuth
// @Router /racks/{id} [put]
func (h *Handler) UpdateRack(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req rackRequest
	if !bind(c, &req) {
		return
	}
	var rack model.Rack
	if err := h.db.First(&rack, id).Error; err != nil {
		statusForDBError(c, err, "机柜不存在")
		return
	}
	if req.RoomID != 0 && req.RoomID != rack.RoomID {
		if !h.roomExists(req.RoomID) {
			errorJSON(c, http.StatusBadRequest, "机房不存在")
			return
		}
		rack.RoomID = req.RoomID
	}
	rack.Name = strings.TrimSpace(req.Name)
	if req.Units > 0 {
		rack.Units = req.Units
	}
	rack.Remark = req.Remark
	if err := h.db.Save(&rack).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新机柜失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, rack)
}

// DeleteRack 删除机柜。
// @Summary 删除机柜
// @Description 删除机柜（admin/asset_manager）
// @Tags racks
// @Param id path int true "机柜 ID"
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /racks/{id} [delete]
func (h *Handler) DeleteRack(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.db.Delete(&model.Rack{}, id).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "删除机柜失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// roomExists 判断机房是否存在。
func (h *Handler) roomExists(roomID uint) bool {
	var count int64
	_ = h.db.Model(&model.DatacenterRoom{}).Where("id = ?", roomID).Count(&count).Error
	return count > 0
}

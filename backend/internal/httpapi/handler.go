package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/config"
	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Handler struct {
	cfg   config.Config
	db    *gorm.DB
	roles []model.Role
	ad    service.ADClient
}

type claims struct {
	UserID uint       `json:"userId"`
	Role   model.Role `json:"role"`
	jwt.RegisteredClaims
}

func NewHandler(cfg config.Config, db *gorm.DB, roles []model.Role, ad service.ADClient) *Handler {
	return &Handler{cfg: cfg, db: db, roles: roles, ad: ad}
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &req) {
		return
	}

	user, ok := h.authenticate(req.Username, req.Password)
	if !ok {
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token, err := h.issueToken(user)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建令牌失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *Handler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": currentUser(c)})
}

func (h *Handler) ListRoles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": h.roles})
}

func (h *Handler) GetADConfig(c *gin.Context) {
	adConfig := h.currentADConfig()
	c.JSON(http.StatusOK, h.adConfigResponse(adConfig))
}

func (h *Handler) SaveADConfig(c *gin.Context) {
	var req adConfigRequest
	if !bind(c, &req) {
		return
	}
	adConfig := h.currentADConfig()
	adConfig.Enabled = req.Enabled
	adConfig.LDAPURL = req.LDAPURL
	adConfig.BaseDN = req.BaseDN
	adConfig.BindDN = req.BindDN
	adConfig.LoginAttribute = defaultString(req.LoginAttribute, "sAMAccountName")
	adConfig.FilterUserObject = req.FilterUserObject
	adConfig.ExcludeDisabled = req.ExcludeDisabled
	adConfig.AdvancedFilter = req.AdvancedFilter
	if req.AdvancedFilter {
		adConfig.UserFilter = defaultString(req.UserFilter, buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled))
	} else {
		adConfig.UserFilter = buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)
	}
	if req.BindPassword != "" {
		encrypted, err := service.EncryptString(req.BindPassword, h.cfg.ConfigKey)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "加密 Bind 密码失败")
			return
		}
		adConfig.EncryptedBindPassword = encrypted
	}
	if adConfig.ID == 0 && adConfig.EncryptedBindPassword == "" {
		errorJSON(c, http.StatusBadRequest, "首次保存 AD 配置必须填写 Bind 密码")
		return
	}
	if err := h.db.Save(&adConfig).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存 AD 配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, h.adConfigResponse(adConfig))
}

func (h *Handler) TestADConnection(c *gin.Context) {
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	if err := h.ad.Test(adConfig, bindPassword); err != nil {
		errorJSON(c, http.StatusBadRequest, "AD 连接测试失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) LookupADUser(c *gin.Context) {
	var req adLookupRequest
	if !bind(c, &req) {
		return
	}
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	info, err := h.ad.LookupUser(adConfig, bindPassword, req.Username)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "查询 AD 用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, info)
}

func (h *Handler) ImportADUser(c *gin.Context) {
	var req adImportRequest
	if !bind(c, &req) {
		return
	}
	adConfig, bindPassword, ok := h.readyADConfig(c)
	if !ok {
		return
	}
	info, err := h.ad.LookupUser(adConfig, bindPassword, req.Username)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "查询 AD 用户失败: "+err.Error())
		return
	}
	var existing model.User
	if err := h.db.Where("username = ?", info.Username).First(&existing).Error; err == nil && existing.AuthSource != "ad" {
		errorJSON(c, http.StatusBadRequest, "同名本地用户已存在，不能导入为 AD 用户")
		return
	}
	role := req.Role
	if role == "" {
		role = model.RoleApplicant
	}
	status := defaultString(req.Status, "active")
	user := existing
	if user.ID == 0 {
		user = model.User{Username: info.Username, Role: role, Status: status, PasswordHash: "AD_AUTH"}
	}
	service.ApplyADInfo(&user, info)
	user.Role = role
	user.Status = status
	if err := h.db.Save(&user).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "导入 AD 用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ListTicketTypeApprovers(c *gin.Context) {
	var items []model.TicketTypeApprover
	if err := h.db.Preload("Approver").Order("type asc").Find(&items).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询审批配置失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) SetTicketTypeApprover(c *gin.Context) {
	ticketType := model.TicketType(c.Param("type"))
	var req ticketTypeApproverRequest
	if !bind(c, &req) {
		return
	}
	var approver model.User
	if err := h.db.First(&approver, req.ApproverID).Error; err != nil {
		statusForDBError(c, err, "审批人不存在")
		return
	}
	if approver.Role != model.RoleApprover && approver.Role != model.RoleAdmin {
		errorJSON(c, http.StatusBadRequest, "审批人必须是审批人或管理员角色")
		return
	}

	var item model.TicketTypeApprover
	err := h.db.Where("type = ?", ticketType).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		item = model.TicketTypeApprover{Type: ticketType, ApproverID: req.ApproverID}
		err = h.db.Create(&item).Error
	} else if err == nil {
		item.ApproverID = req.ApproverID
		err = h.db.Save(&item).Error
	}
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "保存审批配置失败: "+err.Error())
		return
	}
	h.db.Preload("Approver").First(&item, item.ID)
	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.db.Order("id desc").Find(&users).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": users})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req userRequest
	if !bind(c, &req) {
		return
	}
	authSource := defaultString(req.AuthSource, "local")
	if authSource == "ad" {
		errorJSON(c, http.StatusBadRequest, "请通过 AD 导入接口创建 AD 用户")
		return
	}
	if req.Password == "" {
		errorJSON(c, http.StatusBadRequest, "密码不能为空")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "生成密码失败")
		return
	}
	user := model.User{
		Username:     req.Username,
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Email:        req.Email,
		Department:   req.Department,
		Role:         req.Role,
		Status:       defaultString(req.Status, "active"),
		AuthSource:   authSource,
		PasswordHash: string(hash),
	}
	if err := h.db.Create(&user).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var user model.User
	if !h.findByID(c, id, &user) {
		return
	}
	var req userRequest
	if !bind(c, &req) {
		return
	}
	user.Username = req.Username
	user.Name = req.Name
	user.DisplayName = req.DisplayName
	user.Email = req.Email
	user.Department = req.Department
	user.Role = req.Role
	user.Status = defaultString(req.Status, "active")
	if user.AuthSource == "ad" && req.Password != "" {
		errorJSON(c, http.StatusBadRequest, "AD 用户不能在本系统修改密码")
		return
	}
	if user.AuthSource != "ad" && req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "生成密码失败")
			return
		}
		user.PasswordHash = string(hash)
	}
	if err := h.db.Save(&user).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新用户失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) ListAssets(c *gin.Context) {
	var assets []model.Asset
	page, pageSize := assetPagination(c)
	db := applyAssetFilters(h.db.Model(&model.Asset{}), c)
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
	base := applyAssetFilters(h.db.Model(&model.Asset{}), c)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产统计失败")
		return
	}
	var assets []model.Asset
	if err := applyAssetFilters(h.db.Model(&model.Asset{}), c).Select("open_ports", "running_services").Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产统计失败")
		return
	}
	openPortValues := make([]string, 0, len(assets))
	serviceValues := make([]string, 0, len(assets))
	openPortAssetCount := int64(0)
	for _, asset := range assets {
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
		"subnetCount":        len(assetGroupedCounts(applyAssetFilters(h.db.Model(&model.Asset{}), c), "subnet", 1000000)),
		"openPortAssetCount": openPortAssetCount,
		"byAssetType":        assetGroupedCounts(applyAssetFilters(h.db.Model(&model.Asset{}), c), "asset_type", 10),
		"bySubnet":           assetGroupedCounts(applyAssetFilters(h.db.Model(&model.Asset{}), c), "subnet", 10),
		"byOwner":            assetGroupedCounts(applyAssetFilters(h.db.Model(&model.Asset{}), c), "owner", 10),
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

func (h *Handler) ListTickets(c *gin.Context) {
	user := currentUser(c)
	var tickets []model.Ticket
	db := h.db.Preload("Applicant").Preload("Asset").Order("id desc")
	switch c.Query("view") {
	case "todo":
		db = h.applyTodoFilter(db, user)
	case "submitted":
		db = db.Where("applicant_id = ?", user.ID)
	default:
		if user.Role == model.RoleApplicant {
			db = db.Where("applicant_id = ?", user.ID)
		}
	}
	if user.Role == model.RoleApplicant && c.Query("view") == "all" {
		db = db.Where("applicant_id = ?", user.ID)
	}
	if err := db.Find(&tickets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询工单失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": tickets})
}

func (h *Handler) CreateTicket(c *gin.Context) {
	var req ticketRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	ticket := model.Ticket{
		Type:        req.Type,
		Title:       req.Title,
		ApplicantID: user.ID,
		AssetID:     req.AssetID,
		Status:      model.TicketStatusDraft,
		Priority:    defaultPriority(req.Priority),
		Description: req.Description,
	}
	if err := h.db.Create(&ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建工单失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "create", "", ticket.Status, "创建工单")
	c.JSON(http.StatusCreated, ticket)
}

func (h *Handler) GetTicket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Approver").Preload("Executor").Preload("Records.Actor").Preload("Comments.Actor").Preload("Attachments.Uploader").First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return
	}
	if !h.canViewTicket(c, ticket) {
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *Handler) UpdateTicket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if !h.findTicketForUser(c, id, &ticket) {
		return
	}
	if ticket.Status != model.TicketStatusDraft {
		errorJSON(c, http.StatusBadRequest, "只有草稿工单可以编辑")
		return
	}
	var req ticketRequest
	if !bind(c, &req) {
		return
	}
	ticket.Type = req.Type
	ticket.Title = req.Title
	ticket.AssetID = req.AssetID
	ticket.Priority = defaultPriority(req.Priority)
	ticket.Description = req.Description
	if err := h.db.Save(&ticket).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新工单失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, ticket)
}

func (h *Handler) TicketAction(action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c)
		if !ok {
			return
		}
		var req struct {
			Remark string `json:"remark"`
			Result string `json:"result"`
		}
		_ = c.ShouldBindJSON(&req)

		user := currentUser(c)
		var ticket model.Ticket
		if !h.findTicketForUser(c, id, &ticket) {
			return
		}
		next, err := service.Transition(action, ticket.Status, user.Role)
		if err != nil {
			errorJSON(c, http.StatusForbidden, err.Error())
			return
		}
		if action == "submit" && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
			errorJSON(c, http.StatusForbidden, "只能提交自己的工单")
			return
		}
		if action == "submit" {
			approverID, ok := h.defaultApproverID(c, ticket.Type)
			if !ok {
				return
			}
			ticket.ApproverID = &approverID
		}
		if (action == "approve" || action == "reject") && user.Role != model.RoleAdmin {
			if ticket.ApproverID == nil || *ticket.ApproverID != user.ID {
				errorJSON(c, http.StatusForbidden, "只有该工单指定审批人可以处理")
				return
			}
		}
		if action == "close" && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin && user.Role != model.RoleAssetManager {
			errorJSON(c, http.StatusForbidden, "没有关闭该工单的权限")
			return
		}

		from := ticket.Status
		ticket.Status = next
		if action == "approve" {
			ticket.ApproverID = &user.ID
		}
		if action == "start" {
			ticket.ExecutorID = &user.ID
		}
		if action == "complete" {
			ticket.Result = req.Result
		}
		if err := h.db.Save(&ticket).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "更新工单状态失败: "+err.Error())
			return
		}
		h.addRecord(ticket.ID, user.ID, action, from, next, req.Remark)
		c.JSON(http.StatusOK, ticket)
	}
}

func (h *Handler) ListTicketComments(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var comments []model.TicketComment
	if err := h.db.Preload("Actor").Where("ticket_id = ?", ticket.ID).Order("id asc").Find(&comments).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询评论失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": comments})
}

func (h *Handler) CreateTicketComment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var req ticketCommentRequest
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	comment := model.TicketComment{TicketID: ticket.ID, ActorID: user.ID, Content: strings.TrimSpace(req.Content)}
	if comment.Content == "" {
		errorJSON(c, http.StatusBadRequest, "评论不能为空")
		return
	}
	if err := h.db.Create(&comment).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建评论失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "comment", ticket.Status, ticket.Status, comment.Content)
	h.db.Preload("Actor").First(&comment, comment.ID)
	c.JSON(http.StatusCreated, comment)
}

func (h *Handler) ListTicketAttachments(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	var attachments []model.TicketAttachment
	if err := h.db.Preload("Uploader").Where("ticket_id = ?", ticket.ID).Order("id desc").Find(&attachments).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询附件失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": attachments})
}

func (h *Handler) UploadTicketAttachment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "请选择上传文件")
		return
	}
	storedName := uniqueStoredName(file.Filename)
	dir := filepath.Join(h.cfg.AttachmentDir, fmt.Sprint(ticket.ID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		errorJSON(c, http.StatusInternalServerError, "创建附件目录失败")
		return
	}
	storagePath := filepath.Join(dir, storedName)
	if err := c.SaveUploadedFile(file, storagePath); err != nil {
		errorJSON(c, http.StatusInternalServerError, "保存附件失败")
		return
	}
	user := currentUser(c)
	attachment := model.TicketAttachment{
		TicketID:     ticket.ID,
		UploaderID:   user.ID,
		OriginalName: filepath.Base(file.Filename),
		StoredName:   storedName,
		StoragePath:  storagePath,
		Size:         file.Size,
		ContentType:  file.Header.Get("Content-Type"),
	}
	if err := h.db.Create(&attachment).Error; err != nil {
		_ = os.Remove(storagePath)
		errorJSON(c, http.StatusBadRequest, "保存附件元数据失败: "+err.Error())
		return
	}
	h.addRecord(ticket.ID, user.ID, "attach", ticket.Status, ticket.Status, attachment.OriginalName)
	h.db.Preload("Uploader").First(&attachment, attachment.ID)
	c.JSON(http.StatusCreated, attachment)
}

func (h *Handler) DownloadTicketAttachment(c *gin.Context) {
	ticket, ok := h.collaborationTicket(c)
	if !ok {
		return
	}
	attachmentID, err := strconv.ParseUint(c.Param("attachmentId"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效附件 ID")
		return
	}
	var attachment model.TicketAttachment
	if err := h.db.Where("ticket_id = ? AND id = ?", ticket.ID, attachmentID).First(&attachment).Error; err != nil {
		statusForDBError(c, err, "附件不存在")
		return
	}
	c.FileAttachment(attachment.StoragePath, attachment.OriginalName)
}

func (h *Handler) applyTodoFilter(db *gorm.DB, user model.User) *gorm.DB {
	switch user.Role {
	case model.RoleAdmin:
		return db.Where("status IN ?", []model.TicketStatus{
			model.TicketStatusSubmitted,
			model.TicketStatusApproved,
			model.TicketStatusInProgress,
			model.TicketStatusDone,
		})
	case model.RoleApprover:
		return db.Where("status = ? AND approver_id = ?", model.TicketStatusSubmitted, user.ID)
	case model.RoleAssetManager:
		return db.Where("status IN ?", []model.TicketStatus{model.TicketStatusApproved, model.TicketStatusInProgress})
	default:
		return db.Where("status = ? AND applicant_id = ?", model.TicketStatusDone, user.ID)
	}
}

func (h *Handler) defaultApproverID(c *gin.Context, ticketType model.TicketType) (uint, bool) {
	var approverConfig model.TicketTypeApprover
	if err := h.db.Where("type = ?", ticketType).First(&approverConfig).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorJSON(c, http.StatusBadRequest, "该工单类型尚未配置默认审批人")
			return 0, false
		}
		errorJSON(c, http.StatusInternalServerError, "查询默认审批人失败")
		return 0, false
	}
	return approverConfig.ApproverID, true
}

func (h *Handler) collaborationTicket(c *gin.Context) (model.Ticket, bool) {
	id, ok := parseID(c)
	if !ok {
		return model.Ticket{}, false
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return model.Ticket{}, false
	}
	if !h.canCollaborate(c, ticket) {
		return model.Ticket{}, false
	}
	return ticket, true
}

func (h *Handler) canCollaborate(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ApproverID != nil && *ticket.ApproverID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	if user.Role == model.RoleAssetManager {
		return ticket.Status == model.TicketStatusApproved ||
			ticket.Status == model.TicketStatusInProgress ||
			ticket.Status == model.TicketStatusDone
	}
	errorJSON(c, http.StatusForbidden, "没有权限访问该工单协作内容")
	return false
}

func uniqueStoredName(original string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(original))
	}
	return fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), hex.EncodeToString(random[:]), filepath.Base(original))
}

func (h *Handler) authenticate(username, password string) (model.User, bool) {
	var user model.User
	err := h.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return model.User{}, false
	}
	if user.Status != "active" {
		return model.User{}, false
	}
	authMode := strings.ToLower(defaultString(h.cfg.AuthMode, "mixed"))
	if user.AuthSource == "" {
		user.AuthSource = "local"
	}
	if user.AuthSource == "local" {
		if authMode == "ldap" {
			return model.User{}, false
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
			return model.User{}, false
		}
		now := time.Now()
		user.LastLoginAt = &now
		_ = h.db.Save(&user).Error
		return user, true
	}
	if user.AuthSource == "ad" {
		if authMode == "local" {
			return model.User{}, false
		}
		adConfig, bindPassword, ok := h.adConfigForAuth()
		if !ok {
			return model.User{}, false
		}
		info, err := h.ad.Authenticate(adConfig, bindPassword, username, password)
		if err != nil {
			return model.User{}, false
		}
		service.ApplyADInfo(&user, info)
		if err := h.db.Save(&user).Error; err != nil {
			return model.User{}, false
		}
		return user, true
	}
	return model.User{}, false
}

func (h *Handler) adConfigForAuth() (model.ADConfig, string, bool) {
	adConfig := h.currentADConfig()
	if adConfig.ID == 0 || !adConfig.Enabled {
		return model.ADConfig{}, "", false
	}
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.ConfigKey)
	if err != nil {
		return model.ADConfig{}, "", false
	}
	return adConfig, bindPassword, true
}

func (h *Handler) currentADConfig() model.ADConfig {
	var adConfig model.ADConfig
	if err := h.db.First(&adConfig).Error; err != nil {
		return defaultADConfig()
	}
	if adConfig.LoginAttribute == "" {
		adConfig.LoginAttribute = "sAMAccountName"
	}
	if adConfig.UserFilter == "" {
		adConfig.UserFilter = buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)
	}
	return adConfig
}

func (h *Handler) readyADConfig(c *gin.Context) (model.ADConfig, string, bool) {
	adConfig := h.currentADConfig()
	if adConfig.ID == 0 || !adConfig.Enabled {
		errorJSON(c, http.StatusBadRequest, "AD 配置未启用")
		return model.ADConfig{}, "", false
	}
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.ConfigKey)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "AD Bind 密码解密失败")
		return model.ADConfig{}, "", false
	}
	return adConfig, bindPassword, true
}

func (h *Handler) adConfigResponse(adConfig model.ADConfig) gin.H {
	return gin.H{
		"id":               adConfig.ID,
		"enabled":          adConfig.Enabled,
		"ldapUrl":          adConfig.LDAPURL,
		"baseDn":           adConfig.BaseDN,
		"bindDn":           adConfig.BindDN,
		"loginAttribute":   defaultString(adConfig.LoginAttribute, "sAMAccountName"),
		"filterUserObject": adConfig.FilterUserObject,
		"excludeDisabled":  adConfig.ExcludeDisabled,
		"advancedFilter":   adConfig.AdvancedFilter,
		"userFilter":       defaultString(adConfig.UserFilter, buildADUserFilter(adConfig.LoginAttribute, adConfig.FilterUserObject, adConfig.ExcludeDisabled)),
		"hasBindPassword":  adConfig.EncryptedBindPassword != "",
		"createdAt":        adConfig.CreatedAt,
		"updatedAt":        adConfig.UpdatedAt,
	}
}

func defaultADConfig() model.ADConfig {
	return model.ADConfig{
		LoginAttribute:   "sAMAccountName",
		FilterUserObject: true,
		ExcludeDisabled:  true,
		UserFilter:       buildADUserFilter("sAMAccountName", true, true),
	}
}

func buildADUserFilter(loginAttribute string, filterUserObject, excludeDisabled bool) string {
	if loginAttribute == "" {
		loginAttribute = "sAMAccountName"
	}
	parts := []string{fmt.Sprintf("(%s=%%s)", loginAttribute)}
	if filterUserObject {
		parts = append([]string{"(objectClass=user)"}, parts...)
	}
	if excludeDisabled {
		parts = append(parts, "(!(userAccountControl:1.2.840.113556.1.4.803:=2))")
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(&" + strings.Join(parts, "") + ")"
}

func (h *Handler) issueToken(user model.User) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(h.cfg.TokenTTL)),
		},
	})
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			errorJSON(c, http.StatusUnauthorized, "请先登录")
			c.Abort()
			return
		}
		tokenText := strings.TrimPrefix(auth, "Bearer ")
		parsed, err := jwt.ParseWithClaims(tokenText, &claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(h.cfg.JWTSecret), nil
		})
		if err != nil || !parsed.Valid {
			errorJSON(c, http.StatusUnauthorized, "登录已失效")
			c.Abort()
			return
		}
		claim, ok := parsed.Claims.(*claims)
		if !ok {
			errorJSON(c, http.StatusUnauthorized, "登录已失效")
			c.Abort()
			return
		}
		var user model.User
		if err := h.db.First(&user, claim.UserID).Error; err != nil || user.Status != "active" {
			errorJSON(c, http.StatusUnauthorized, "用户不可用")
			c.Abort()
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

func (h *Handler) RequireAnyRole(roles ...model.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		for _, role := range roles {
			if user.Role == role {
				c.Next()
				return
			}
		}
		errorJSON(c, http.StatusForbidden, "没有权限执行该操作")
		c.Abort()
	}
}

func currentUser(c *gin.Context) model.User {
	user, _ := c.Get("user")
	typed, _ := user.(model.User)
	return typed
}

func (h *Handler) findByID(c *gin.Context, id uint, out interface{}) bool {
	if err := h.db.First(out, id).Error; err != nil {
		statusForDBError(c, err, "资源不存在")
		return false
	}
	return true
}

func (h *Handler) findTicketForUser(c *gin.Context, id uint, ticket *model.Ticket) bool {
	if err := h.db.First(ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return false
	}
	return h.canViewTicket(c, *ticket)
}

func (h *Handler) canViewTicket(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role != model.RoleApplicant || ticket.ApplicantID == user.ID {
		return true
	}
	errorJSON(c, http.StatusForbidden, "没有权限查看该工单")
	return false
}

func (h *Handler) addRecord(ticketID, actorID uint, action string, from, to model.TicketStatus, remark string) {
	_ = h.db.Create(&model.TicketRecord{
		TicketID:   ticketID,
		ActorID:    actorID,
		Action:     action,
		FromStatus: from,
		ToStatus:   to,
		Remark:     remark,
	}).Error
}

func (h *Handler) audit(actorID uint, entity string, entityID uint, action, detail string) {
	_ = h.db.Create(&model.AuditLog{
		ActorID:  actorID,
		Entity:   entity,
		EntityID: entityID,
		Action:   action,
		Detail:   detail,
	}).Error
}

func parseID(c *gin.Context) (uint, bool) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "无效 ID")
		return 0, false
	}
	return uint(id64), true
}

func bind(c *gin.Context, out interface{}) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		errorJSON(c, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return false
	}
	return true
}

func statusForDBError(c *gin.Context, err error, notFound string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		errorJSON(c, http.StatusNotFound, notFound)
		return
	}
	errorJSON(c, http.StatusInternalServerError, err.Error())
}

func errorJSON(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultPriority(value model.Priority) model.Priority {
	if value == "" {
		return model.PriorityNormal
	}
	return value
}

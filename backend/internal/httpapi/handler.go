package httpapi

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
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
	cfg      config.Config
	db       *gorm.DB
	roles    []model.Role
	ad       service.ADClient
	archiver service.TicketArchiver
	mail     service.MailSender
}

type claims struct {
	UserID uint       `json:"userId"`
	Role   model.Role `json:"role"`
	jwt.RegisteredClaims
}

func NewHandler(cfg config.Config, db *gorm.DB, roles []model.Role, ad service.ADClient, archiver service.TicketArchiver, mailSender service.MailSender) *Handler {
	return &Handler{cfg: cfg, db: db, roles: roles, ad: ad, archiver: archiver, mail: mailSender}
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
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
		encrypted, err := service.EncryptString(req.BindPassword, h.cfg.Security.ConfigEncryptionKey)
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

func (h *Handler) GetMailConfig(c *gin.Context) {
	mailConfig := h.currentMailConfig()
	c.JSON(http.StatusOK, h.mailConfigResponse(mailConfig))
}

func (h *Handler) SaveMailConfig(c *gin.Context) {
	var req mailConfigRequest
	if !bind(c, &req) {
		return
	}
	mailConfig := h.currentMailConfig()
	mailConfig.Enabled = req.Enabled
	mailConfig.SMTPHost = strings.TrimSpace(req.SMTPHost)
	mailConfig.SMTPPort = req.SMTPPort
	if mailConfig.SMTPPort == 0 {
		mailConfig.SMTPPort = 25
	}
	mailConfig.Username = strings.TrimSpace(req.Username)
	mailConfig.FromAddress = strings.TrimSpace(req.FromAddress)
	mailConfig.FromName = strings.TrimSpace(defaultString(req.FromName, "资产管理系统"))
	mailConfig.UseTLS = req.UseTLS
	mailConfig.StartTLS = req.StartTLS
	if mailConfig.Enabled {
		if mailConfig.SMTPHost == "" || mailConfig.FromAddress == "" {
			errorJSON(c, http.StatusBadRequest, "启用邮件通知必须填写 SMTP 地址和发件邮箱")
			return
		}
		if _, err := mail.ParseAddress(mailConfig.FromAddress); err != nil {
			errorJSON(c, http.StatusBadRequest, "发件邮箱格式无效")
			return
		}
	}
	if req.Password != "" {
		encrypted, err := service.EncryptString(req.Password, h.cfg.Security.ConfigEncryptionKey)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "加密 SMTP 密码失败")
			return
		}
		mailConfig.EncryptedPassword = encrypted
	}
	if err := h.db.Save(&mailConfig).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "保存邮件配置失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, h.mailConfigResponse(mailConfig))
}

func (h *Handler) TestMailConfig(c *gin.Context) {
	var req mailTestRequest
	_ = c.ShouldBindJSON(&req)
	mailConfig, password, ok := h.readyMailConfig(c)
	if !ok {
		return
	}
	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" {
		recipient = mailConfig.FromAddress
	}
	address, err := mail.ParseAddress(recipient)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "测试收件邮箱格式无效")
		return
	}
	if err := h.mail.Send(mailConfig, password, service.MailMessage{
		To:      []mail.Address{*address},
		Subject: "资产管理系统邮件测试",
		Body:    "这是一封来自资产管理系统的邮件测试。收到此邮件表示 SMTP 配置可用。",
	}); err != nil {
		errorJSON(c, http.StatusBadRequest, "邮件发送测试失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
	db := h.db.Preload("Applicant").Preload("Asset").Preload("WorkflowSteps.Approvers").Order("id desc")
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
		Type:            req.Type,
		Title:           req.Title,
		ApplicantID:     user.ID,
		AssetID:         req.AssetID,
		Status:          model.TicketStatusDraft,
		Priority:        defaultPriority(req.Priority),
		Description:     req.Description,
		DeviceType:      req.DeviceType,
		DeviceName:      req.DeviceName,
		IPAddress:       req.IPAddress,
		OpenPorts:       req.OpenPorts,
		RunningServices: req.RunningServices,
		AppVersion:      req.AppVersion,
		Manufacturer:    req.Manufacturer,
		Antivirus:       req.Antivirus,
		ChangeContent:   req.ChangeContent,
		Impact:          req.Impact,
		Remark:          req.Remark,
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
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Approver").Preload("Executor").
		Preload("Records.Actor").Preload("Comments.Actor").Preload("Attachments.Uploader").
		Preload("WorkflowSteps.Actor").Preload("WorkflowSteps.Approvers.User", func(db *gorm.DB) *gorm.DB { return db.Order("id asc") }).
		Preload("WorkflowSteps", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc") }).
		First(&ticket, id).Error; err != nil {
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
	if ticket.Status != model.TicketStatusDraft && ticket.Status != model.TicketStatusRejected {
		errorJSON(c, http.StatusBadRequest, "只有草稿或驳回工单可以编辑")
		return
	}
	var req ticketRequest
	if !bind(c, &req) {
		return
	}
	applyTicketRequest(&ticket, req)
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
		var req ticketActionRequest
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
		if (action == "submit" || action == "accept") && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
			errorJSON(c, http.StatusForbidden, "只能提交自己的工单")
			return
		}
		if action == "submit" {
			if !h.createWorkflowSnapshot(c, &ticket) {
				return
			}
			h.addRecord(ticket.ID, user.ID, action, model.TicketStatusDraft, ticket.Status, req.Remark)
			h.notifyCurrentApprovers(ticket.ID)
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "approve" {
			if !h.approveCurrentStep(c, &ticket, user, req.Remark) {
				return
			}
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "reject" {
			from := ticket.Status
			ticket.Status = next
			if !h.rejectCurrentStep(c, &ticket, user, req.Remark) {
				return
			}
			h.addRecord(ticket.ID, user.ID, action, from, next, req.Remark)
			c.JSON(http.StatusOK, ticket)
			return
		}
		if action == "accept" && ticket.ApplicantID != user.ID && user.Role != model.RoleAdmin {
			errorJSON(c, http.StatusForbidden, "只有申请人可以验收关闭该工单")
			return
		}

		from := ticket.Status
		ticket.Status = next
		if action == "start" {
			ticket.ExecutorID = &user.ID
		}
		if action == "complete" {
			ticket.Result = req.Result
		}
		if action == "accept" {
			ticket.AcceptanceResult = defaultString(req.AcceptanceResult, req.Remark)
			archiveNo, archivePath, ok := h.closeTicketWithArchive(c, &ticket)
			if !ok {
				return
			}
			ticket.ArchiveNo = archiveNo
			ticket.ArchivePath = archivePath
			now := time.Now()
			ticket.ArchivedAt = &now
		}
		if err := h.db.Save(&ticket).Error; err != nil {
			errorJSON(c, http.StatusBadRequest, "更新工单状态失败: "+err.Error())
			return
		}
		h.addRecord(ticket.ID, user.ID, action, from, next, req.Remark)
		c.JSON(http.StatusOK, ticket)
	}
}

func (h *Handler) DownloadTicketArchive(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return
	}
	if !h.canDownloadArchive(c, ticket) {
		return
	}
	if ticket.Status != model.TicketStatusClosed {
		errorJSON(c, http.StatusBadRequest, "工单关闭后才能下载归档 PDF")
		return
	}
	if ticket.ArchivePath == "" || ticket.ArchiveNo == "" {
		errorJSON(c, http.StatusBadRequest, "工单归档 PDF 尚未生成")
		return
	}
	c.FileAttachment(ticket.ArchivePath, ticket.ArchiveNo+".pdf")
}

func (h *Handler) DownloadTicketArchives(c *gin.Context) {
	var req ticketArchiveBatchRequest
	if !bind(c, &req) {
		return
	}
	if len(req.IDs) == 0 {
		errorJSON(c, http.StatusBadRequest, "请选择要下载的工单")
		return
	}
	if len(req.IDs) > 100 {
		errorJSON(c, http.StatusBadRequest, "单次最多下载 100 个归档 PDF")
		return
	}
	ids := uniqueUint(req.IDs)
	var tickets []model.Ticket
	if err := h.db.Where("id IN ?", ids).Find(&tickets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询工单归档失败")
		return
	}
	if len(tickets) != len(ids) {
		errorJSON(c, http.StatusNotFound, "包含不存在的工单")
		return
	}
	ticketByID := make(map[uint]model.Ticket, len(tickets))
	for _, ticket := range tickets {
		ticketByID[ticket.ID] = ticket
	}
	ordered := make([]model.Ticket, 0, len(ids))
	for _, id := range ids {
		ticket := ticketByID[id]
		if !h.canDownloadArchive(c, ticket) {
			return
		}
		if ticket.Status != model.TicketStatusClosed {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 关闭后才能下载归档 PDF", ticket.ID))
			return
		}
		if ticket.ArchivePath == "" || ticket.ArchiveNo == "" {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 归档 PDF 尚未生成", ticket.ID))
			return
		}
		if _, err := os.Stat(ticket.ArchivePath); err != nil {
			errorJSON(c, http.StatusBadRequest, fmt.Sprintf("工单 #%d 归档 PDF 文件不存在", ticket.ID))
			return
		}
		ordered = append(ordered, ticket)
	}

	filename := fmt.Sprintf("ticket-archives-%s.zip", time.Now().Format("20060102150405"))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()
	usedNames := map[string]int{}
	for _, ticket := range ordered {
		name := ticket.ArchiveNo + ".pdf"
		if usedNames[name] > 0 {
			name = fmt.Sprintf("%s-%d.pdf", ticket.ArchiveNo, usedNames[name]+1)
		}
		usedNames[ticket.ArchiveNo+".pdf"]++
		if err := addFileToZip(zipWriter, ticket.ArchivePath, name); err != nil {
			errorJSON(c, http.StatusInternalServerError, "打包归档 PDF 失败: "+err.Error())
			return
		}
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
	dir := filepath.Join(h.cfg.Storage.AttachmentDir, fmt.Sprint(ticket.ID))
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
			model.TicketStatusPendingApproval,
			model.TicketStatusApproved,
			model.TicketStatusInProgress,
			model.TicketStatusPendingAcceptance,
			model.TicketStatusRejected,
		})
	case model.RoleApprover:
		return db.Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.ticket_id = tickets.id AND ticket_workflow_steps.id = tickets.current_workflow_step_id").
			Joins("JOIN ticket_workflow_step_approvers ON ticket_workflow_step_approvers.step_id = ticket_workflow_steps.id").
			Where("tickets.status = ? AND ticket_workflow_step_approvers.user_id = ?", model.TicketStatusPendingApproval, user.ID)
	case model.RoleAssetManager:
		return db.Where("status IN ?", []model.TicketStatus{model.TicketStatusApproved, model.TicketStatusInProgress})
	default:
		return db.Where("status IN ? AND applicant_id = ?", []model.TicketStatus{model.TicketStatusPendingAcceptance, model.TicketStatusRejected}, user.ID)
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
			ticket.Status == model.TicketStatusPendingAcceptance
	}
	errorJSON(c, http.StatusForbidden, "没有权限访问该工单协作内容")
	return false
}

func (h *Handler) canDownloadArchive(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	var count int64
	_ = h.db.Model(&model.TicketWorkflowStepApprover{}).
		Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.id = ticket_workflow_step_approvers.step_id").
		Where("ticket_workflow_steps.ticket_id = ? AND ticket_workflow_step_approvers.user_id = ?", ticket.ID, user.ID).
		Count(&count).Error
	if count > 0 {
		return true
	}
	errorJSON(c, http.StatusForbidden, "没有权限下载该工单归档")
	return false
}

func applyTicketRequest(ticket *model.Ticket, req ticketRequest) {
	ticket.Type = req.Type
	ticket.Title = req.Title
	ticket.AssetID = req.AssetID
	ticket.Priority = defaultPriority(req.Priority)
	ticket.Description = req.Description
	ticket.DeviceType = req.DeviceType
	ticket.DeviceName = req.DeviceName
	ticket.IPAddress = req.IPAddress
	ticket.OpenPorts = req.OpenPorts
	ticket.RunningServices = req.RunningServices
	ticket.AppVersion = req.AppVersion
	ticket.Manufacturer = req.Manufacturer
	ticket.Antivirus = req.Antivirus
	ticket.ChangeContent = req.ChangeContent
	ticket.Impact = req.Impact
	ticket.Remark = req.Remark
}

func (h *Handler) closeTicketWithArchive(c *gin.Context, ticket *model.Ticket) (string, string, bool) {
	var full model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Executor").Preload("Records.Actor").
		Preload("WorkflowSteps.Actor").Preload("WorkflowSteps", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc") }).
		First(&full, ticket.ID).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "加载归档数据失败")
		return "", "", false
	}
	full.Status = ticket.Status
	full.Result = ticket.Result
	full.AcceptanceResult = ticket.AcceptanceResult
	archiveNo, archivePath, err := h.archiver.Generate(c.Request.Context(), service.TicketArchiveData{Ticket: full}, h.cfg.Storage.TicketTemplatePath, h.cfg.Storage.TicketArchiveDir, h.cfg.Storage.LibreOfficeBin)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "生成归档 PDF 失败: "+err.Error())
		return "", "", false
	}
	if err := h.writeBackTicketAsset(ticket); err != nil {
		_ = os.Remove(archivePath)
		errorJSON(c, http.StatusBadRequest, "写回资产台账失败: "+err.Error())
		return "", "", false
	}
	return archiveNo, archivePath, true
}

func (h *Handler) writeBackTicketAsset(ticket *model.Ticket) error {
	if ticket.AssetID == nil {
		return nil
	}
	var asset model.Asset
	if err := h.db.First(&asset, *ticket.AssetID).Error; err != nil {
		return err
	}
	if ticket.DeviceType != "" {
		asset.AssetType = ticket.DeviceType
	}
	if ticket.DeviceName != "" {
		asset.Hostname = ticket.DeviceName
	}
	if ticket.IPAddress != "" {
		asset.IP = ticket.IPAddress
	}
	if ticket.OpenPorts != "" {
		asset.OpenPorts = ticket.OpenPorts
	}
	if ticket.RunningServices != "" {
		asset.RunningServices = ticket.RunningServices
	}
	if ticket.AppVersion != "" {
		asset.AppVersion = ticket.AppVersion
	}
	if ticket.Manufacturer != "" {
		asset.Manufacturer = ticket.Manufacturer
	}
	return h.db.Save(&asset).Error
}

func uniqueStoredName(original string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(original))
	}
	return fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), hex.EncodeToString(random[:]), filepath.Base(original))
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, archiveName string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	writer, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
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
	authMode := strings.ToLower(defaultString(h.cfg.Auth.Mode, "mixed"))
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
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.Security.ConfigEncryptionKey)
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
	bindPassword, err := service.DecryptString(adConfig.EncryptedBindPassword, h.cfg.Security.ConfigEncryptionKey)
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

func (h *Handler) currentMailConfig() model.MailConfig {
	var mailConfig model.MailConfig
	if err := h.db.First(&mailConfig).Error; err != nil {
		return defaultMailConfig()
	}
	if mailConfig.SMTPPort == 0 {
		mailConfig.SMTPPort = 25
	}
	if mailConfig.FromName == "" {
		mailConfig.FromName = "资产管理系统"
	}
	return mailConfig
}

func defaultMailConfig() model.MailConfig {
	return model.MailConfig{
		SMTPPort: 25,
		FromName: "资产管理系统",
		StartTLS: true,
	}
}

func (h *Handler) readyMailConfig(c *gin.Context) (model.MailConfig, string, bool) {
	mailConfig := h.currentMailConfig()
	if mailConfig.ID == 0 || !mailConfig.Enabled {
		errorJSON(c, http.StatusBadRequest, "邮件配置未启用")
		return model.MailConfig{}, "", false
	}
	password, err := h.mailConfigPassword(mailConfig)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "SMTP 密码解密失败")
		return model.MailConfig{}, "", false
	}
	return mailConfig, password, true
}

func (h *Handler) mailConfigPassword(mailConfig model.MailConfig) (string, error) {
	if mailConfig.EncryptedPassword == "" {
		return "", nil
	}
	return service.DecryptString(mailConfig.EncryptedPassword, h.cfg.Security.ConfigEncryptionKey)
}

func (h *Handler) mailConfigResponse(mailConfig model.MailConfig) gin.H {
	return gin.H{
		"id":          mailConfig.ID,
		"enabled":     mailConfig.Enabled,
		"smtpHost":    mailConfig.SMTPHost,
		"smtpPort":    mailConfig.SMTPPort,
		"username":    mailConfig.Username,
		"fromAddress": mailConfig.FromAddress,
		"fromName":    mailConfig.FromName,
		"useTls":      mailConfig.UseTLS,
		"startTls":    mailConfig.StartTLS,
		"hasPassword": mailConfig.EncryptedPassword != "",
		"createdAt":   mailConfig.CreatedAt,
		"updatedAt":   mailConfig.UpdatedAt,
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
	return token.SignedString([]byte(h.cfg.Security.JWTSecret))
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
			return []byte(h.cfg.Security.JWTSecret), nil
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

func (h *Handler) notifyCurrentApprovers(ticketID uint) {
	mailConfig := h.currentMailConfig()
	if mailConfig.ID == 0 || !mailConfig.Enabled {
		return
	}
	password, err := h.mailConfigPassword(mailConfig)
	if err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "SMTP 密码解密失败")
		return
	}
	var ticket model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").First(&ticket, ticketID).Error; err != nil {
		return
	}
	if ticket.CurrentWorkflowStepID == nil {
		return
	}
	var step model.TicketWorkflowStep
	if err := h.db.Preload("Approvers.User").First(&step, *ticket.CurrentWorkflowStepID).Error; err != nil {
		return
	}
	recipients := make([]mail.Address, 0, len(step.Approvers))
	for _, approver := range step.Approvers {
		address, err := mail.ParseAddress(strings.TrimSpace(approver.User.Email))
		if err != nil {
			continue
		}
		if approver.User.Name != "" {
			address.Name = approver.User.Name
		}
		recipients = append(recipients, *address)
	}
	if len(recipients) == 0 {
		h.addSystemRecord(ticketID, "mail_skipped", "当前审批节点没有可用审批人邮箱")
		return
	}
	message := service.MailMessage{
		To:      recipients,
		Subject: fmt.Sprintf("待审批工单 #%d：%s", ticket.ID, ticket.Title),
		Body:    approvalMailBody(ticket, step),
	}
	if err := h.mail.Send(mailConfig, password, message); err != nil {
		h.addSystemRecord(ticketID, "mail_failed", "审批通知邮件发送失败: "+err.Error())
		return
	}
	h.addSystemRecord(ticketID, "mail_sent", fmt.Sprintf("已通知审批节点 %s，共 %d 人", step.Name, len(recipients)))
}

func (h *Handler) addSystemRecord(ticketID uint, action, remark string) {
	var ticket model.Ticket
	if err := h.db.First(&ticket, ticketID).Error; err != nil {
		return
	}
	h.addRecord(ticketID, ticket.ApplicantID, action, ticket.Status, ticket.Status, remark)
}

func approvalMailBody(ticket model.Ticket, step model.TicketWorkflowStep) string {
	lines := []string{
		"您好，",
		"",
		"有一张工单等待您审批。",
		"",
		fmt.Sprintf("工单编号：#%d", ticket.ID),
		"工单标题：" + ticket.Title,
		"审批节点：" + step.Name,
		"工单类型：" + string(ticket.Type),
		"优先级：" + string(ticket.Priority),
		"申请人：" + defaultString(ticket.Applicant.Name, ticket.Applicant.Username),
	}
	if ticket.Asset != nil {
		lines = append(lines, "关联资产："+defaultString(ticket.Asset.Hostname, ticket.Asset.IP))
	}
	if strings.TrimSpace(ticket.Description) != "" {
		lines = append(lines, "", "申请说明：", ticket.Description)
	}
	lines = append(lines, "", "请登录资产管理系统处理。")
	return strings.Join(lines, "\n")
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

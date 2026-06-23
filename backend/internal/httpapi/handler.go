package httpapi

import (
	"errors"
	"fmt"
	"net/http"
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
}

type claims struct {
	UserID uint       `json:"userId"`
	Role   model.Role `json:"role"`
	jwt.RegisteredClaims
}

func NewHandler(cfg config.Config, db *gorm.DB, roles []model.Role) *Handler {
	return &Handler{cfg: cfg, db: db, roles: roles}
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &req) {
		return
	}

	var user model.User
	if err := h.db.Where("username = ? AND status = ?", req.Username, "active").First(&user).Error; err != nil {
		errorJSON(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
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
		Role:         req.Role,
		Status:       defaultString(req.Status, "active"),
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
	user.Role = req.Role
	user.Status = defaultString(req.Status, "active")
	if req.Password != "" {
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
	q := strings.TrimSpace(c.Query("q"))
	db := h.db.Order("id desc")
	if q != "" {
		like := "%" + q + "%"
		db = db.Where("asset_no LIKE ? OR hostname LIKE ? OR ip LIKE ? OR owner LIKE ?", like, like, like, like)
	}
	if err := db.Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询资产失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": assets})
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
	if user.Role == model.RoleApplicant {
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
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Approver").Preload("Executor").Preload("Records.Actor").First(&ticket, id).Error; err != nil {
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

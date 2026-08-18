package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"asset-registration-management-system/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type loginAttempt struct {
	Failures   int
	WindowFrom time.Time
	BlockedTo  time.Time
}

type loginRateLimiter struct {
	mu         sync.Mutex
	attempts   map[string]loginAttempt
	max        int
	window     time.Duration
	block      time.Duration
	lastPruned time.Time
}

func newLoginRateLimiter(max int, window, block time.Duration) *loginRateLimiter {
	return &loginRateLimiter{attempts: map[string]loginAttempt{}, max: max, window: window, block: block}
}

func (l *loginRateLimiter) keys(username, ip string) []string {
	return []string{"account:" + username, "ip:" + ip}
}

func (l *loginRateLimiter) allow(username, ip string, now time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range l.keys(username, ip) {
		item := l.attempts[key]
		if item.BlockedTo.After(now) {
			return item.BlockedTo.Sub(now), false
		}
	}
	return 0, true
}

func (l *loginRateLimiter) failure(username, ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, key := range l.keys(username, ip) {
		item := l.attempts[key]
		if item.WindowFrom.IsZero() || now.Sub(item.WindowFrom) > l.window {
			item = loginAttempt{WindowFrom: now}
		}
		item.Failures++
		if item.Failures >= l.max {
			item.BlockedTo = now.Add(l.block)
			item.Failures = 0
			item.WindowFrom = now
		}
		l.attempts[key] = item
	}
	if now.Sub(l.lastPruned) > time.Hour {
		for key, item := range l.attempts {
			if now.Sub(item.WindowFrom) > l.window && !item.BlockedTo.After(now) {
				delete(l.attempts, key)
			}
		}
		l.lastPruned = now
	}
}

func (l *loginRateLimiter) success(username string) {
	l.mu.Lock()
	delete(l.attempts, "account:"+username)
	l.mu.Unlock()
}

func jwtKeyID(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:6])
}

func newSessionID() (string, error) {
	value := make([]byte, 24)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (h *Handler) issueToken(c *gin.Context, user model.User) (string, error) {
	now := time.Now().UTC()
	sessionID, err := newSessionID()
	if err != nil {
		return "", err
	}
	if user.SessionVersion == 0 {
		user.SessionVersion = 1
		if err := h.db.Model(&user).Update("session_version", 1).Error; err != nil {
			return "", err
		}
	}
	keyID := jwtKeyID(h.cfg.Security.JWTSecret)
	expiresAt := now.Add(h.cfg.TokenTTL)
	session := model.AuthSession{
		ID:                sessionID,
		UserID:            user.ID,
		KeyID:             keyID,
		SessionVersion:    user.SessionVersion,
		ClientIP:          c.ClientIP(),
		UserAgent:         strings.TrimSpace(c.GetHeader("User-Agent")),
		ReauthenticatedAt: now,
		LastSeenAt:        now,
		ExpiresAt:         expiresAt,
	}
	if err := h.db.Create(&session).Error; err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID:         user.ID,
		Role:           user.Role,
		SessionID:      session.ID,
		SessionVersion: user.SessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        session.ID,
			Subject:   fmt.Sprint(user.ID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	token.Header["kid"] = keyID
	signed, err := token.SignedString([]byte(h.cfg.Security.JWTSecret))
	if err != nil {
		_ = h.db.Delete(&session).Error
	}
	return signed, err
}

func (h *Handler) signingKeys() map[string]string {
	keys := map[string]string{jwtKeyID(h.cfg.Security.JWTSecret): h.cfg.Security.JWTSecret}
	for _, secret := range h.cfg.Security.JWTPreviousSecrets {
		secret = strings.TrimSpace(secret)
		if secret != "" {
			keys[jwtKeyID(secret)] = secret
		}
	}
	return keys
}

func (h *Handler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			h.abortAuth(c, "请先登录")
			return
		}
		tokenText := strings.TrimPrefix(auth, "Bearer ")
		parsed, err := jwt.ParseWithClaims(tokenText, &claims{}, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Method.Alg())
			}
			keyID, _ := token.Header["kid"].(string)
			secret, ok := h.signingKeys()[keyID]
			if !ok {
				return nil, fmt.Errorf("unknown signing key")
			}
			return []byte(secret), nil
		})
		if err != nil || !parsed.Valid {
			h.abortAuth(c, "登录已失效")
			return
		}
		claim, ok := parsed.Claims.(*claims)
		if !ok || claim.SessionID == "" {
			h.abortAuth(c, "登录已失效")
			return
		}
		var session model.AuthSession
		if err := h.db.First(&session, "id = ? AND user_id = ?", claim.SessionID, claim.UserID).Error; err != nil ||
			session.RevokedAt != nil || !session.ExpiresAt.After(time.Now()) ||
			session.SessionVersion != claim.SessionVersion {
			h.abortAuth(c, "登录已失效")
			return
		}
		var user model.User
		if err := h.db.First(&user, claim.UserID).Error; err != nil || user.Status != "active" ||
			user.SessionVersion != claim.SessionVersion {
			h.abortAuth(c, "用户不可用")
			return
		}
		if time.Since(session.LastSeenAt) >= time.Minute {
			now := time.Now().UTC()
			_ = h.db.Model(&session).Update("last_seen_at", now).Error
			session.LastSeenAt = now
		}
		c.Set("user", user)
		c.Set("session", session)
		c.Next()
	}
}

func (h *Handler) abortAuth(c *gin.Context, message string) {
	errorJSON(c, http.StatusUnauthorized, message)
	c.Abort()
}

func revokeUserSessions(db *gorm.DB, userID uint, reason, exceptID string) error {
	now := time.Now().UTC()
	query := db.Model(&model.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID)
	if exceptID != "" {
		query = query.Where("id <> ?", exceptID)
	}
	return query.Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error
}

func (h *Handler) revokeSession(sessionID string, userID uint, reason string) error {
	now := time.Now().UTC()
	return h.db.Model(&model.AuthSession{}).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL", sessionID, userID).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error
}

func (h *Handler) ListSessions(c *gin.Context) {
	user := currentUser(c)
	current := currentSession(c)
	var sessions []model.AuthSession
	if err := h.db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).
		Order("created_at DESC").Find(&sessions).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询会话失败")
		return
	}
	items := make([]gin.H, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, gin.H{"session": session, "current": session.ID == current.ID})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	if err := h.revokeSession(c.Param("id"), currentUser(c).ID, "user_revoked"); err != nil {
		errorJSON(c, http.StatusBadRequest, "撤销会话失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *Handler) RevokeAllSessions(c *gin.Context) {
	if err := revokeUserSessions(h.db, currentUser(c).ID, "revoke_all", ""); err != nil {
		errorJSON(c, http.StatusInternalServerError, "撤销会话失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true})
}

func (h *Handler) Reauthenticate(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if !bind(c, &req) {
		return
	}
	user := currentUser(c)
	authenticated, ok := h.authenticate(user.Username, req.Password)
	if !ok || authenticated.ID != user.ID {
		errorJSON(c, http.StatusUnauthorized, "密码验证失败")
		return
	}
	now := time.Now().UTC()
	session := currentSession(c)
	if err := h.db.Model(&model.AuthSession{}).Where("id = ?", session.ID).
		Update("reauthenticated_at", now).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "二次认证失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"reauthenticatedAt": now})
}

func (h *Handler) RequireRecentAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := currentSession(c)
		validAfter := time.Now().Add(-time.Duration(h.cfg.Security.SensitiveReauthMinutes) * time.Minute)
		if session.ReauthenticatedAt.Before(validAfter) {
			c.AbortWithStatusJSON(http.StatusPreconditionRequired, gin.H{
				"error": "此操作需要二次认证",
				"code":  "reauth_required",
			})
			return
		}
		c.Next()
	}
}

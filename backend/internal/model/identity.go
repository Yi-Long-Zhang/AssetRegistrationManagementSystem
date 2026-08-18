package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID                 uint           `json:"id" gorm:"primaryKey"`
	Username           string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Name               string         `json:"name" gorm:"size:128;not null"`
	DisplayName        string         `json:"displayName" gorm:"size:128"`
	Email              string         `json:"email" gorm:"size:128"`
	Department         string         `json:"department" gorm:"size:128"`
	Role               Role           `json:"role" gorm:"size:32;not null;index"`
	Status             string         `json:"status" gorm:"size:32;not null;default:active"`
	AuthSource         string         `json:"authSource" gorm:"size:32;not null;default:local;index"`
	ADDN               string         `json:"adDn" gorm:"size:512"`
	ProxyUserID        *uint          `json:"proxyUserId" gorm:"index"` // 审批代理：设置为本人审批的工单由代理处理
	LastLoginAt        *time.Time     `json:"lastLoginAt"`
	FailedAttempts     int            `json:"-" gorm:"not null;default:0"`                      // 连续登录失败次数
	LockedUntil        *time.Time     `json:"-" gorm:"index"`                                   // 登录锁定截止时间（暴力破解防护）
	MustChangePassword bool           `json:"mustChangePassword" gorm:"not null;default:false"` // 首次登录强制改密
	PasswordHash       string         `json:"-" gorm:"size:255;not null"`
	SessionVersion     uint64         `json:"-" gorm:"not null;default:1"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

// AuthSession is the server-side authority for a JWT.
type AuthSession struct {
	ID                string     `json:"id" gorm:"primaryKey;size:64"`
	UserID            uint       `json:"userId" gorm:"not null;index"`
	User              User       `json:"user,omitempty" gorm:"foreignKey:UserID"`
	KeyID             string     `json:"keyId" gorm:"size:32;not null;index"`
	SessionVersion    uint64     `json:"-" gorm:"not null"`
	ClientIP          string     `json:"clientIp" gorm:"size:64"`
	UserAgent         string     `json:"userAgent" gorm:"size:512"`
	ReauthenticatedAt time.Time  `json:"reauthenticatedAt"`
	LastSeenAt        time.Time  `json:"lastSeenAt"`
	ExpiresAt         time.Time  `json:"expiresAt" gorm:"not null;index"`
	RevokedAt         *time.Time `json:"revokedAt" gorm:"index"`
	RevokedReason     string     `json:"revokedReason,omitempty" gorm:"size:128"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ADConfig struct {
	ID                    uint      `json:"id" gorm:"primaryKey"`
	Enabled               bool      `json:"enabled" gorm:"not null;default:false"`
	LDAPURL               string    `json:"ldapUrl" gorm:"size:255;not null"`
	BaseDN                string    `json:"baseDn" gorm:"size:512;not null"`
	BindDN                string    `json:"bindDn" gorm:"size:512;not null"`
	EncryptedBindPassword string    `json:"-" gorm:"type:text"`
	LoginAttribute        string    `json:"loginAttribute" gorm:"size:64"`
	FilterUserObject      bool      `json:"filterUserObject"`
	ExcludeDisabled       bool      `json:"excludeDisabled"`
	AdvancedFilter        bool      `json:"advancedFilter"`
	UserFilter            string    `json:"userFilter" gorm:"size:255;not null"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

// MailConfig 邮件服务配置（系统级，单行）
type MailConfig struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	Enabled           bool      `json:"enabled" gorm:"not null;default:false"`
	SMTPHost          string    `json:"smtpHost" gorm:"size:255;not null"`
	SMTPPort          int       `json:"smtpPort" gorm:"not null;default:25"`
	Username          string    `json:"username" gorm:"size:255"`
	EncryptedPassword string    `json:"-" gorm:"type:text"`
	FromAddress       string    `json:"fromAddress" gorm:"size:255;not null"`
	FromName          string    `json:"fromName" gorm:"size:128"`
	UseTLS            bool      `json:"useTls" gorm:"not null;default:false"`
	StartTLS          bool      `json:"startTls" gorm:"not null;default:true"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// IMConfig 群机器人通知配置（系统级，单行）：钉钉/企微/飞书。
type IMConfig struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Enabled         bool      `json:"enabled" gorm:"not null;default:false"`
	Platform        string    `json:"platform" gorm:"size:32;not null;default:'dingtalk'"` // dingtalk/wecom/feishu
	Webhook         string    `json:"webhook" gorm:"type:text;not null"`                   // 群机器人 webhook 地址
	Secret          string    `json:"-" gorm:"size:255"`                                   // 加签/回调验签密钥（AES-GCM 加密存储）
	EncryptedSecret bool      `json:"-" gorm:"not null;default:false"`                     // Secret 是否已加密
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// IMCallbackConfig IM 回调验签配置（档位 2 自建应用，系统级单行）。
// 敏感字段（AppSecret/Token/EncodingAESKey）AES-GCM 加密存储。
type IMCallbackConfig struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Platform       string    `json:"platform" gorm:"size:32;not null;default:'dingtalk'"` // dingtalk/wecom/feishu
	Enabled        bool      `json:"enabled" gorm:"not null;default:false"`
	AppSecret      string    `json:"-" gorm:"type:text"`              // 钉钉 AppSecret / 飞书应用 secret
	CorpID         string    `json:"corpId" gorm:"size:128"`          // 企微 CorpID（非敏感）
	Token          string    `json:"-" gorm:"size:255"`               // 企微 Token / 飞书 verification token
	EncodingAESKey string    `json:"-" gorm:"size:128"`               // 企微 EncodingAESKey
	Encrypted      bool      `json:"-" gorm:"not null;default:false"` // 敏感字段是否已加密
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// IMBinding IM 用户与系统用户映射（用于 IM 回调鉴权）。
type IMBinding struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"userId" gorm:"not null;uniqueIndex"`
	User      User      `json:"user" gorm:"foreignKey:UserID"`
	Platform  string    `json:"platform" gorm:"size:32;not null;uniqueIndex:idx_im_binding_platform_user"`  // dingtalk/wecom/feishu
	IMUserID  string    `json:"imUserId" gorm:"size:128;not null;uniqueIndex:idx_im_binding_platform_user"` // 平台侧用户标识（openId/userId）
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

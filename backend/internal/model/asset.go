package model

import (
	"time"

	"gorm.io/gorm"
)

type Asset struct {
	ID                 uint              `json:"id" gorm:"primaryKey"`
	AssetNo            string            `json:"assetNo" gorm:"uniqueIndex;size:64;not null"`
	SequenceNo         string            `json:"sequenceNo" gorm:"size:64"`
	AssetType          string            `json:"assetType" gorm:"size:64"`
	Hostname           string            `json:"hostname" gorm:"size:128;not null"`
	IP                 string            `json:"ip" gorm:"size:64;not null;index"`
	MACAddress         string            `json:"macAddress" gorm:"size:128;index"`
	ManagementIP       string            `json:"managementIp" gorm:"size:64"`
	SerialNo           string            `json:"serialNo" gorm:"size:128;index"`
	Manufacturer       string            `json:"manufacturer" gorm:"size:128"`
	Model              string            `json:"model" gorm:"size:128"`
	Location           string            `json:"location" gorm:"size:128"`
	Rack               string            `json:"rack" gorm:"size:128"`
	RackPosition       string            `json:"rackPosition" gorm:"size:64"`
	OS                 string            `json:"os" gorm:"size:128"`
	OSVersion          string            `json:"osVersion" gorm:"size:128"`
	CPU                string            `json:"cpu" gorm:"size:128"`
	Memory             string            `json:"memory" gorm:"size:128"`
	Disk               string            `json:"disk" gorm:"size:128"`
	OpenPorts          string            `json:"openPorts" gorm:"type:text"`
	RunningServices    string            `json:"runningServices" gorm:"type:text"`
	AppVersion         string            `json:"appVersion" gorm:"type:text"`
	Subnet             string            `json:"subnet" gorm:"size:128;index"`
	BusinessSystem     string            `json:"businessSystem" gorm:"size:128;index"`
	Environment        string            `json:"environment" gorm:"size:64"`
	Department         string            `json:"department" gorm:"size:128"`
	Owner              string            `json:"owner" gorm:"size:128"`
	MaintenanceVendor  string            `json:"maintenanceVendor" gorm:"size:128"`
	PurchaseDate       *time.Time        `json:"purchaseDate"`
	WarrantyExpireDate *time.Time        `json:"warrantyExpireDate"`
	Status             AssetStatus       `json:"status" gorm:"size:32;not null;index"`
	OnlineStatus       AssetOnlineStatus `json:"onlineStatus" gorm:"size:32;not null;default:unknown;index"`
	LastSeenAt         *time.Time        `json:"lastSeenAt"`
	DiscoveredAt       *time.Time        `json:"discoveredAt"`
	AdditionalIPs      string            `json:"additionalIPs" gorm:"type:text"` // 关联 IP 列表，逗号分隔（多网卡）
	OnlineDate         *time.Time        `json:"onlineDate"`
	Remark             string            `json:"remark" gorm:"type:text"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt    `json:"-" gorm:"index"`
}

// Credential 服务器凭据（账号/密码或密钥），敏感字段 AES-GCM 加密存储。
type Credential struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	AssetID         *uint          `json:"assetId" gorm:"index"`
	Asset           *Asset         `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Name            string         `json:"name" gorm:"size:128;not null"` // 凭据用途名，如 "root 登录"
	Username        string         `json:"username" gorm:"size:128;not null"`
	Type            string         `json:"type" gorm:"size:32;not null;default:'ssh'"` // ssh/rdp/database/app/other
	EncryptedSecret string         `json:"-" gorm:"type:text;not null"`                // AES-GCM 密文（不对外暴露）
	Remark          string         `json:"remark" gorm:"type:text"`
	LastAccessedAt  *time.Time     `json:"lastAccessedAt"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// SoftwareLicense 软件许可证台账（许可证密钥 AES-GCM 加密存储）。
type SoftwareLicense struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"size:128;not null"`                     // 软件名
	Vendor       string         `json:"vendor" gorm:"size:128"`                            // 厂商
	Type         string         `json:"type" gorm:"size:32;not null;default:'commercial'"` // commercial/open-source/subscription/other
	LicenseKey   string         `json:"-" gorm:"type:text"`                                // 许可证密钥密文（不对外暴露）
	Encrypted    bool           `json:"-" gorm:"not null;default:false"`                   // 密钥是否已加密
	TotalSeats   int            `json:"totalSeats" gorm:"not null;default:0"`              // 授权数量
	UsedSeats    int            `json:"usedSeats" gorm:"not null;default:0"`               // 已用数量
	ExpireDate   *time.Time     `json:"expireDate"`                                        // 到期日
	PurchaseDate *time.Time     `json:"purchaseDate"`
	AssetID      *uint          `json:"assetId" gorm:"index"`
	Asset        *Asset         `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Remark       string         `json:"remark" gorm:"type:text"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

// LicenseAttachment 软件许可附件（授权书/合同扫描件等）。
type LicenseAttachment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	LicenseID    uint      `json:"licenseId" gorm:"not null;index"`
	UploaderID   uint      `json:"uploaderId" gorm:"not null;index"`
	Uploader     User      `json:"uploader" gorm:"foreignKey:UploaderID"`
	OriginalName string    `json:"originalName" gorm:"size:255;not null"`
	StoredName   string    `json:"storedName" gorm:"size:255;not null"`
	StoragePath  string    `json:"-" gorm:"size:512;not null"`
	Size         int64     `json:"size" gorm:"not null"`
	ContentType  string    `json:"contentType" gorm:"size:128"`
	CreatedAt    time.Time `json:"createdAt"`
}

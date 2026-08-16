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
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
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

type Ticket struct {
	ID                      uint                 `json:"id" gorm:"primaryKey"`
	Type                    TicketType           `json:"type" gorm:"size:32;not null;index"`
	Title                   string               `json:"title" gorm:"size:200;not null"`
	ApplicantID             uint                 `json:"applicantId" gorm:"not null;index"`
	Applicant               User                 `json:"applicant" gorm:"foreignKey:ApplicantID"`
	LicenseID               *uint                `json:"licenseId" gorm:"index"` // 关联软件许可（到期自动续费工单专用，可选）
	AssetID                 *uint                `json:"assetId" gorm:"index"`
	Asset                   *Asset               `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Status                  TicketStatus         `json:"status" gorm:"size:32;not null;index"`
	ApproverID              *uint                `json:"approverId" gorm:"index"`
	Approver                *User                `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	ExecutorID              *uint                `json:"executorId" gorm:"index"`
	Executor                *User                `json:"executor,omitempty" gorm:"foreignKey:ExecutorID"`
	CurrentWorkflowStepID   *uint                `json:"currentWorkflowStepId" gorm:"index"`
	CurrentWorkflowStepName string               `json:"currentWorkflowStepName" gorm:"size:128"`
	ArchiveNo               *string              `json:"archiveNo" gorm:"size:64;uniqueIndex"` // 归档号（关闭归档时生成，草稿为空）
	ArchivePath             string               `json:"-" gorm:"size:512"`
	ArchivedAt              *time.Time           `json:"archivedAt"`
	Priority                Priority             `json:"priority" gorm:"size:32;not null"`
	Description             string               `json:"description" gorm:"type:text"`
	Result                  string               `json:"result" gorm:"type:text"`
	AcceptanceResult        string               `json:"acceptanceResult" gorm:"type:text"`
	DeviceType              string               `json:"deviceType" gorm:"size:64"`
	DeviceName              string               `json:"deviceName" gorm:"size:128"`
	IPAddress               string               `json:"ipAddress" gorm:"size:64;index"`
	OpenPorts               string               `json:"openPorts" gorm:"type:text"`
	RunningServices         string               `json:"runningServices" gorm:"type:text"`
	AppVersion              string               `json:"appVersion" gorm:"type:text"`
	Manufacturer            string               `json:"manufacturer" gorm:"size:128"`
	Antivirus               string               `json:"antivirus" gorm:"size:128"`
	ChangeContent           string               `json:"changeContent" gorm:"type:text"`
	Impact                  string               `json:"impact" gorm:"type:text"`
	Remark                  string               `json:"remark" gorm:"type:text"`
	CreatedAt               time.Time            `json:"createdAt"`
	UpdatedAt               time.Time            `json:"updatedAt"`
	SLAApprovalDeadline     *time.Time           `json:"slaApprovalDeadline"`   // SLA：审批截止时间（submit 时按流程类型写入）
	SLACompletionDeadline   *time.Time           `json:"slaCompletionDeadline"` // SLA：执行完成截止时间（start 时写入）
	SLAStartedAt            *time.Time           `json:"slaStartedAt"`          // SLA：进入执行阶段时间（用于剩余时限展示）
	SLAOverdueNotified      bool                 `json:"slaOverdueNotified"`    // SLA：是否已发送超时通知（防重复）
	Records                 []TicketRecord       `json:"records,omitempty"`
	Comments                []TicketComment      `json:"comments,omitempty"`
	Attachments             []TicketAttachment   `json:"attachments,omitempty"`
	WorkflowSteps           []TicketWorkflowStep `json:"workflowSteps,omitempty"`
	Assets                  []TicketAsset        `json:"assets,omitempty" gorm:"foreignKey:TicketID"` // 关联资产（多资产）
}

// TicketAsset 工单与资产多对多关联。
type TicketAsset struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TicketID  uint      `json:"ticketId" gorm:"not null;index"`
	AssetID   uint      `json:"assetId" gorm:"not null;index"`
	Asset     Asset     `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	CreatedAt time.Time `json:"createdAt"`
}

type TicketWorkflow struct {
	ID              uint                 `json:"id" gorm:"primaryKey"`
	Type            TicketType           `json:"type" gorm:"uniqueIndex;size:32;not null"`
	Name            string               `json:"name" gorm:"size:128;not null"`
	Enabled         bool                 `json:"enabled" gorm:"not null;default:true"`
	ApprovalHours   *int                 `json:"approvalHours"`   // SLA：审批时限（小时），nil=不启用
	CompletionHours *int                 `json:"completionHours"` // SLA：执行完成时限（小时），nil=不启用
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	Nodes           []TicketWorkflowNode `json:"nodes,omitempty" gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE"`
}

type TicketWorkflowNode struct {
	ID         uint                         `json:"id" gorm:"primaryKey"`
	WorkflowID uint                         `json:"workflowId" gorm:"not null;index"`
	Name       string                       `json:"name" gorm:"size:128;not null"`
	SortOrder  int                          `json:"sortOrder" gorm:"not null;index"`
	CreatedAt  time.Time                    `json:"createdAt"`
	UpdatedAt  time.Time                    `json:"updatedAt"`
	Approvers  []TicketWorkflowNodeApprover `json:"approvers,omitempty" gorm:"foreignKey:NodeID;constraint:OnDelete:CASCADE"`
}

type TicketWorkflowNodeApprover struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	NodeID    uint      `json:"nodeId" gorm:"not null;index"`
	UserID    uint      `json:"userId" gorm:"not null;index"`
	User      User      `json:"user" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}

type TicketWorkflowStep struct {
	ID        uint                         `json:"id" gorm:"primaryKey"`
	TicketID  uint                         `json:"ticketId" gorm:"not null;index"`
	Name      string                       `json:"name" gorm:"size:128;not null"`
	SortOrder int                          `json:"sortOrder" gorm:"not null;index"`
	Status    string                       `json:"status" gorm:"size:32;not null;index"`
	ActorID   *uint                        `json:"actorId" gorm:"index"`
	Actor     *User                        `json:"actor,omitempty" gorm:"foreignKey:ActorID"`
	Remark    string                       `json:"remark" gorm:"type:text"`
	ActedAt   *time.Time                   `json:"actedAt"`
	CreatedAt time.Time                    `json:"createdAt"`
	UpdatedAt time.Time                    `json:"updatedAt"`
	Approvers []TicketWorkflowStepApprover `json:"approvers,omitempty" gorm:"foreignKey:StepID;constraint:OnDelete:CASCADE"`
}

type TicketWorkflowStepApprover struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	StepID    uint      `json:"stepId" gorm:"not null;index"`
	UserID    uint      `json:"userId" gorm:"not null;index"`
	User      User      `json:"user" gorm:"foreignKey:UserID"`
	CreatedAt time.Time `json:"createdAt"`
}

type TicketTypeApprover struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	Type       TicketType `json:"type" gorm:"uniqueIndex;size:32;not null"`
	ApproverID uint       `json:"approverId" gorm:"not null;index"`
	Approver   User       `json:"approver" gorm:"foreignKey:ApproverID"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type TicketComment struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TicketID  uint      `json:"ticketId" gorm:"not null;index"`
	ActorID   uint      `json:"actorId" gorm:"not null;index"`
	Actor     User      `json:"actor" gorm:"foreignKey:ActorID"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"createdAt"`
}

type TicketAttachment struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	TicketID     uint      `json:"ticketId" gorm:"not null;index"`
	UploaderID   uint      `json:"uploaderId" gorm:"not null;index"`
	Uploader     User      `json:"uploader" gorm:"foreignKey:UploaderID"`
	OriginalName string    `json:"originalName" gorm:"size:255;not null"`
	StoredName   string    `json:"storedName" gorm:"size:255;not null"`
	StoragePath  string    `json:"-" gorm:"size:512;not null"`
	Size         int64     `json:"size" gorm:"not null"`
	ContentType  string    `json:"contentType" gorm:"size:128"`
	CreatedAt    time.Time `json:"createdAt"`
}

type TicketRecord struct {
	ID         uint         `json:"id" gorm:"primaryKey"`
	TicketID   uint         `json:"ticketId" gorm:"not null;index"`
	ActorID    uint         `json:"actorId" gorm:"not null;index"`
	Actor      User         `json:"actor" gorm:"foreignKey:ActorID"`
	Action     string       `json:"action" gorm:"size:64;not null"`
	FromStatus TicketStatus `json:"fromStatus" gorm:"size:32"`
	ToStatus   TicketStatus `json:"toStatus" gorm:"size:32"`
	Remark     string       `json:"remark" gorm:"type:text"`
	CreatedAt  time.Time    `json:"createdAt"`
}

// StocktakeTask 资产盘点单。
type StocktakeTask struct {
	ID        uint            `json:"id" gorm:"primaryKey"`
	Name      string          `json:"name" gorm:"size:128;not null"`
	Status    string          `json:"status" gorm:"size:32;not null;default:in_progress;index"` // in_progress / closed
	CreatorID uint            `json:"creatorId" gorm:"not null;index"`
	Creator   User            `json:"creator,omitempty" gorm:"foreignKey:CreatorID"`
	Remark    string          `json:"remark" gorm:"type:text"`
	CreatedAt time.Time       `json:"createdAt"`
	ClosedAt  *time.Time      `json:"closedAt"`
	Items     []StocktakeItem `json:"items,omitempty" gorm:"foreignKey:TaskID"`
}

// StocktakeItem 盘点明细（创建盘点单时对资产快照）。
type StocktakeItem struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	TaskID    uint       `json:"taskId" gorm:"not null;index"`
	AssetID   uint       `json:"assetId" gorm:"not null;index"`
	Asset     Asset      `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Result    string     `json:"result" gorm:"size:32;not null;default:pending;index"` // pending / matched / missing
	Remark    string     `json:"remark" gorm:"type:text"`
	CheckedAt *time.Time `json:"checkedAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// DatacenterRoom 机房。
type DatacenterRoom struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128;not null;uniqueIndex"`
	Location  string    `json:"location" gorm:"size:128"` // 位置/地址
	Remark    string    `json:"remark" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Racks     []Rack    `json:"racks,omitempty" gorm:"foreignKey:RoomID"`
}

// Rack 机柜。
type Rack struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	RoomID    uint           `json:"roomId" gorm:"not null;index"`
	Room      DatacenterRoom `json:"room,omitempty" gorm:"foreignKey:RoomID"`
	Name      string         `json:"name" gorm:"size:128;not null"`
	Units     int            `json:"units" gorm:"not null;default:42"` // U 位数，默认 42
	Remark    string         `json:"remark" gorm:"type:text"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type AuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ActorID   uint      `json:"actorId" gorm:"not null;index"`
	Actor     User      `json:"actor" gorm:"foreignKey:ActorID"`
	Entity    string    `json:"entity" gorm:"size:64;not null;index"`
	EntityID  uint      `json:"entityId" gorm:"not null;index"`
	Action    string    `json:"action" gorm:"size:64;not null"`
	Detail    string    `json:"detail" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
}

// DiscoveryRule 资产发现规则：定义扫描目标与行为
type DiscoveryRule struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	Name            string     `json:"name" gorm:"size:128;not null"`
	Targets         string     `json:"targets" gorm:"type:text;not null"` // CIDR/IP 列表，逗号分隔
	Ports           string     `json:"ports" gorm:"size:255"`             // 端口列表，如 "22,80,443"；空=使用默认端口
	ProbePorts      string     `json:"probePorts" gorm:"size:255"`        // 两阶段扫描探活端口；空=使用配置默认
	ServiceDetect   bool       `json:"serviceDetect" gorm:"not null;default:false"`
	IntervalMinutes int        `json:"intervalMinutes" gorm:"not null;default:60"`
	AutoAdopt       bool       `json:"autoAdopt" gorm:"not null;default:false"`
	AutoApply       bool       `json:"autoApply" gorm:"not null;default:false"`
	AutoTicket      bool       `json:"autoTicket" gorm:"not null;default:false"`  // 高风险变更自动生成变更工单
	Incremental     bool       `json:"incremental" gorm:"not null;default:false"` // 增量扫描：仅重扫上次发现的存活主机
	ScanWindowStart string     `json:"scanWindowStart" gorm:"size:5"`             // 扫描时段开始 "HH:MM"，空=全天
	ScanWindowEnd   string     `json:"scanWindowEnd" gorm:"size:5"`               // 扫描时段结束 "HH:MM"，空=全天
	Enabled         bool       `json:"enabled" gorm:"not null;default:true"`
	LastRunAt       *time.Time `json:"lastRunAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// InspectionRule 定期巡检规则：按频率自动生成巡检工单（草稿，需人工提交审批）。
type InspectionRule struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Name        string     `json:"name" gorm:"size:128;not null"`                   // 规则名称
	Description string     `json:"description" gorm:"type:text"`                    // 巡检内容说明
	Frequency   string     `json:"frequency" gorm:"size:32;not null;default:daily"` // daily/weekly/monthly
	DayOfWeek   int        `json:"dayOfWeek" gorm:"not null;default:1"`             // weekly：0=周日 ~ 6=周六
	DayOfMonth  int        `json:"dayOfMonth" gorm:"not null;default:1"`            // monthly：1-31
	TimeOfDay   string     `json:"timeOfDay" gorm:"size:8;not null;default:09:00"`  // "HH:MM" 执行时间
	AssigneeID  uint       `json:"assigneeId" gorm:"not null;index"`                // 巡检执行人
	Assignee    User       `json:"assignee,omitempty" gorm:"foreignKey:AssigneeID"`
	Enabled     bool       `json:"enabled" gorm:"not null;default:true"`
	LastRunAt   *time.Time `json:"lastRunAt"` // 上次生成巡检工单时间
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// DiscoveryRun 一次发现执行记录
type DiscoveryRun struct {
	ID           uint               `json:"id" gorm:"primaryKey"`
	RuleID       uint               `json:"ruleId" gorm:"not null;index"`
	Rule         DiscoveryRule      `json:"rule,omitempty" gorm:"foreignKey:RuleID"`
	Trigger      string             `json:"trigger" gorm:"size:32;not null;default:manual"`
	Status       DiscoveryRunStatus `json:"status" gorm:"size:32;not null;index"`
	NewCount     int                `json:"newCount"`
	ChangedCount int                `json:"changedCount"`
	OfflineCount int                `json:"offlineCount"`
	OnlineCount  int                `json:"onlineCount"`
	TotalHosts   int                `json:"totalHosts"`
	Error        string             `json:"error" gorm:"type:text"`
	StartedAt    time.Time          `json:"startedAt"`
	FinishedAt   *time.Time         `json:"finishedAt"`
	Hosts        []DiscoveredHost   `json:"hosts,omitempty" gorm:"foreignKey:RunID;constraint:OnDelete:CASCADE"`
}

// DiscoveredHost 单台主机发现结果
type DiscoveredHost struct {
	ID             uint                `json:"id" gorm:"primaryKey"`
	RunID          uint                `json:"runId" gorm:"not null;index"`
	IP             string              `json:"ip" gorm:"size:64;not null;index"`
	MAC            string              `json:"mac" gorm:"size:64"`
	Hostname       string              `json:"hostname" gorm:"size:255"`
	Status         string              `json:"status" gorm:"size:32;not null;default:up"`
	OpenPorts      string              `json:"openPorts" gorm:"type:text"`
	Services       string              `json:"services" gorm:"type:text"`
	OS             string              `json:"os" gorm:"size:255"`
	ChangeType     DiscoveryChangeType `json:"changeType" gorm:"size:32;not null;index"`
	ChangeRisk     ChangeRiskLevel     `json:"changeRisk" gorm:"size:16;not null;default:low"` // 变更风险级别：low=可自动应用 / high=需人工确认
	MatchedAssetID *uint               `json:"matchedAssetId" gorm:"index"`
	MatchedAsset   *Asset              `json:"matchedAsset,omitempty" gorm:"foreignKey:MatchedAssetID"`
	DiffSummary    string              `json:"diffSummary" gorm:"type:text"`
	Adopted        bool                `json:"adopted" gorm:"not null;default:false"`
	Applied        bool                `json:"applied" gorm:"not null;default:false"`
	CreatedAt      time.Time           `json:"createdAt"`
}

// AssetSnapshot 资产字段快照与相对上一快照的变更 diff
type AssetSnapshot struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	AssetID      uint           `json:"assetId" gorm:"not null;index"`
	Source       SnapshotSource `json:"source" gorm:"size:32;not null"`
	ChangeType   string         `json:"changeType" gorm:"size:32;not null"`
	SnapshotJSON string         `json:"-" gorm:"type:text;not null"`
	DiffJSON     string         `json:"-" gorm:"type:text"`
	DiffSummary  string         `json:"diffSummary" gorm:"type:text"`
	CreatedBy    *uint          `json:"createdBy" gorm:"index"`
	CreatedAt    time.Time      `json:"createdAt"`
}

// IPSegment IP 地址池网段（IP 规划与占用管理）
type IPSegment struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128;not null"` // 网段名称，如「生产网段 A」
	CIDR        string    `json:"cidr" gorm:"size:64;not null"`  // CIDR，如 10.0.0.0/24
	Description string    `json:"description" gorm:"size:255"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

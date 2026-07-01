package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Name         string         `json:"name" gorm:"size:128;not null"`
	DisplayName  string         `json:"displayName" gorm:"size:128"`
	Email        string         `json:"email" gorm:"size:128"`
	Department   string         `json:"department" gorm:"size:128"`
	Role         Role           `json:"role" gorm:"size:32;not null;index"`
	Status       string         `json:"status" gorm:"size:32;not null;default:active"`
	AuthSource   string         `json:"authSource" gorm:"size:32;not null;default:local;index"`
	ADDN         string         `json:"adDn" gorm:"size:512"`
	LastLoginAt  *time.Time     `json:"lastLoginAt"`
	PasswordHash string         `json:"-" gorm:"size:255;not null"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
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

type Asset struct {
	ID                 uint           `json:"id" gorm:"primaryKey"`
	AssetNo            string         `json:"assetNo" gorm:"uniqueIndex;size:64;not null"`
	SequenceNo         string         `json:"sequenceNo" gorm:"size:64"`
	AssetType          string         `json:"assetType" gorm:"size:64"`
	Hostname           string         `json:"hostname" gorm:"size:128;not null"`
	IP                 string         `json:"ip" gorm:"size:64;not null;index"`
	MACAddress         string         `json:"macAddress" gorm:"size:128;index"`
	ManagementIP       string         `json:"managementIp" gorm:"size:64"`
	SerialNo           string         `json:"serialNo" gorm:"size:128;index"`
	Manufacturer       string         `json:"manufacturer" gorm:"size:128"`
	Model              string         `json:"model" gorm:"size:128"`
	Location           string         `json:"location" gorm:"size:128"`
	Rack               string         `json:"rack" gorm:"size:128"`
	RackPosition       string         `json:"rackPosition" gorm:"size:64"`
	OS                 string         `json:"os" gorm:"size:128"`
	OSVersion          string         `json:"osVersion" gorm:"size:128"`
	CPU                string         `json:"cpu" gorm:"size:128"`
	Memory             string         `json:"memory" gorm:"size:128"`
	Disk               string         `json:"disk" gorm:"size:128"`
	OpenPorts          string         `json:"openPorts" gorm:"type:text"`
	RunningServices    string         `json:"runningServices" gorm:"type:text"`
	AppVersion         string         `json:"appVersion" gorm:"type:text"`
	Subnet             string         `json:"subnet" gorm:"size:128;index"`
	BusinessSystem     string         `json:"businessSystem" gorm:"size:128;index"`
	Environment        string         `json:"environment" gorm:"size:64"`
	Department         string         `json:"department" gorm:"size:128"`
	Owner              string         `json:"owner" gorm:"size:128"`
	MaintenanceVendor  string         `json:"maintenanceVendor" gorm:"size:128"`
	PurchaseDate       *time.Time     `json:"purchaseDate"`
	WarrantyExpireDate *time.Time     `json:"warrantyExpireDate"`
	Status             AssetStatus    `json:"status" gorm:"size:32;not null;index"`
	OnlineDate         *time.Time     `json:"onlineDate"`
	Remark             string         `json:"remark" gorm:"type:text"`
	CreatedAt          time.Time      `json:"createdAt"`
	UpdatedAt          time.Time      `json:"updatedAt"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

type Ticket struct {
	ID                      uint                 `json:"id" gorm:"primaryKey"`
	Type                    TicketType           `json:"type" gorm:"size:32;not null;index"`
	Title                   string               `json:"title" gorm:"size:200;not null"`
	ApplicantID             uint                 `json:"applicantId" gorm:"not null;index"`
	Applicant               User                 `json:"applicant" gorm:"foreignKey:ApplicantID"`
	AssetID                 *uint                `json:"assetId" gorm:"index"`
	Asset                   *Asset               `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Status                  TicketStatus         `json:"status" gorm:"size:32;not null;index"`
	ApproverID              *uint                `json:"approverId" gorm:"index"`
	Approver                *User                `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	ExecutorID              *uint                `json:"executorId" gorm:"index"`
	Executor                *User                `json:"executor,omitempty" gorm:"foreignKey:ExecutorID"`
	CurrentWorkflowStepID   *uint                `json:"currentWorkflowStepId" gorm:"index"`
	CurrentWorkflowStepName string               `json:"currentWorkflowStepName" gorm:"size:128"`
	ArchiveNo               string               `json:"archiveNo" gorm:"size:64;uniqueIndex"`
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
	Records                 []TicketRecord       `json:"records,omitempty"`
	Comments                []TicketComment      `json:"comments,omitempty"`
	Attachments             []TicketAttachment   `json:"attachments,omitempty"`
	WorkflowSteps           []TicketWorkflowStep `json:"workflowSteps,omitempty"`
}

type TicketWorkflow struct {
	ID        uint                 `json:"id" gorm:"primaryKey"`
	Type      TicketType           `json:"type" gorm:"uniqueIndex;size:32;not null"`
	Name      string               `json:"name" gorm:"size:128;not null"`
	Enabled   bool                 `json:"enabled" gorm:"not null;default:true"`
	CreatedAt time.Time            `json:"createdAt"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Nodes     []TicketWorkflowNode `json:"nodes,omitempty" gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE"`
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

type AuditLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ActorID   uint      `json:"actorId" gorm:"not null;index"`
	Entity    string    `json:"entity" gorm:"size:64;not null;index"`
	EntityID  uint      `json:"entityId" gorm:"not null;index"`
	Action    string    `json:"action" gorm:"size:64;not null"`
	Detail    string    `json:"detail" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
}

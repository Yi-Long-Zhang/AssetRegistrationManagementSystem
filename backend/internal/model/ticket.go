package model

import (
	"time"
)

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

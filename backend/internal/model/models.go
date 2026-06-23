package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Username     string         `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Name         string         `json:"name" gorm:"size:128;not null"`
	Role         Role           `json:"role" gorm:"size:32;not null;index"`
	Status       string         `json:"status" gorm:"size:32;not null;default:active"`
	PasswordHash string         `json:"-" gorm:"size:255;not null"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

type Asset struct {
	ID         uint           `json:"id" gorm:"primaryKey"`
	AssetNo    string         `json:"assetNo" gorm:"uniqueIndex;size:64;not null"`
	Hostname   string         `json:"hostname" gorm:"size:128;not null"`
	IP         string         `json:"ip" gorm:"size:64;not null;index"`
	Location   string         `json:"location" gorm:"size:128"`
	Rack       string         `json:"rack" gorm:"size:128"`
	OS         string         `json:"os" gorm:"size:128"`
	CPU        string         `json:"cpu" gorm:"size:128"`
	Memory     string         `json:"memory" gorm:"size:128"`
	Disk       string         `json:"disk" gorm:"size:128"`
	Owner      string         `json:"owner" gorm:"size:128"`
	Status     AssetStatus    `json:"status" gorm:"size:32;not null;index"`
	OnlineDate *time.Time     `json:"onlineDate"`
	Remark     string         `json:"remark" gorm:"type:text"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
}

type Ticket struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Type        TicketType     `json:"type" gorm:"size:32;not null;index"`
	Title       string         `json:"title" gorm:"size:200;not null"`
	ApplicantID uint           `json:"applicantId" gorm:"not null;index"`
	Applicant   User           `json:"applicant" gorm:"foreignKey:ApplicantID"`
	AssetID     *uint          `json:"assetId" gorm:"index"`
	Asset       *Asset         `json:"asset,omitempty" gorm:"foreignKey:AssetID"`
	Status      TicketStatus   `json:"status" gorm:"size:32;not null;index"`
	ApproverID  *uint          `json:"approverId" gorm:"index"`
	Approver    *User          `json:"approver,omitempty" gorm:"foreignKey:ApproverID"`
	ExecutorID  *uint          `json:"executorId" gorm:"index"`
	Executor    *User          `json:"executor,omitempty" gorm:"foreignKey:ExecutorID"`
	Priority    Priority       `json:"priority" gorm:"size:32;not null"`
	Description string         `json:"description" gorm:"type:text"`
	Result      string         `json:"result" gorm:"type:text"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Records     []TicketRecord `json:"records,omitempty"`
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

package model

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleAssetManager Role = "asset_manager"
	RoleApprover     Role = "approver"
	RoleApplicant    Role = "applicant"
)

func AllRoles() []Role {
	return []Role{RoleAdmin, RoleAssetManager, RoleApprover, RoleApplicant}
}

type AssetStatus string

const (
	AssetStatusPending      AssetStatus = "pending"
	AssetStatusInUse        AssetStatus = "in_use"
	AssetStatusMaintenance  AssetStatus = "maintenance"
	AssetStatusRetired      AssetStatus = "retired"
	AssetStatusDecommission AssetStatus = "decommissioned"
)

type TicketType string

const (
	TicketTypeAssetRegister TicketType = "asset_register"
	TicketTypeAssetChange   TicketType = "asset_change"
	TicketTypeAssetRetire   TicketType = "asset_retire"
	TicketTypeMaintenance   TicketType = "maintenance"
)

type TicketStatus string

const (
	TicketStatusDraft             TicketStatus = "draft"
	TicketStatusPendingApproval   TicketStatus = "pending_approval"
	TicketStatusApproved          TicketStatus = "approved"
	TicketStatusRejected          TicketStatus = "rejected"
	TicketStatusInProgress        TicketStatus = "in_progress"
	TicketStatusPendingAcceptance TicketStatus = "pending_acceptance"
	TicketStatusClosed            TicketStatus = "closed"
	TicketStatusCancelled         TicketStatus = "cancelled"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

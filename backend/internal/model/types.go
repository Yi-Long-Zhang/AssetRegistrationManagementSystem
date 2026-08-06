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

// AssetOnlineStatus 资产在线状态（与生命周期 AssetStatus 分离）
type AssetOnlineStatus string

const (
	AssetOnlineStatusOnline  AssetOnlineStatus = "online"
	AssetOnlineStatusOffline AssetOnlineStatus = "offline"
	AssetOnlineStatusUnknown AssetOnlineStatus = "unknown"
)

// DiscoveryRunStatus 发现运行状态
type DiscoveryRunStatus string

const (
	DiscoveryRunStatusRunning DiscoveryRunStatus = "running"
	DiscoveryRunStatusSuccess DiscoveryRunStatus = "success"
	DiscoveryRunStatusFailed  DiscoveryRunStatus = "failed"
)

// DiscoveryChangeType 单台主机的发现比对结果
type DiscoveryChangeType string

const (
	DiscoveryChangeNew     DiscoveryChangeType = "new"
	DiscoveryChangeChanged DiscoveryChangeType = "changed"
	DiscoveryChangeOffline DiscoveryChangeType = "offline"
	DiscoveryChangeOnline  DiscoveryChangeType = "online"
	DiscoveryChangeNone    DiscoveryChangeType = "none"
)

// SnapshotSource 资产快照来源
type SnapshotSource string

const (
	SnapshotSourceDiscovery SnapshotSource = "discovery"
	SnapshotSourceTicket    SnapshotSource = "ticket"
	SnapshotSourceImport    SnapshotSource = "import"
	SnapshotSourceManual    SnapshotSource = "manual"
)

package model

import "strings"

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
	TicketTypeInspection    TicketType = "inspection"
	TicketTypeLicenseRenew  TicketType = "license_renew" // 软件许可续费
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

type BackgroundTaskStatus string

const (
	BackgroundTaskQueued    BackgroundTaskStatus = "queued"
	BackgroundTaskRunning   BackgroundTaskStatus = "running"
	BackgroundTaskSucceeded BackgroundTaskStatus = "succeeded"
	BackgroundTaskFailed    BackgroundTaskStatus = "failed"
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

// ChangeRiskLevel 变更风险级别：low=低风险（仅端口新增等，可自动应用），high=高风险（需人工确认）
type ChangeRiskLevel string

const (
	ChangeRiskLow  ChangeRiskLevel = "low"
	ChangeRiskHigh ChangeRiskLevel = "high"
)

// SnapshotSource 资产快照来源
type SnapshotSource string

const (
	SnapshotSourceDiscovery SnapshotSource = "discovery"
	SnapshotSourceTicket    SnapshotSource = "ticket"
	SnapshotSourceImport    SnapshotSource = "import"
	SnapshotSourceManual    SnapshotSource = "manual"
)

// 软件许可证类型
const (
	LicenseTypeCommercial   = "commercial"   // 商业授权
	LicenseTypeOpenSource   = "open-source"  // 开源
	LicenseTypeSubscription = "subscription" // 订阅制
	LicenseTypeOther        = "other"        // 其他
)

// NormalizeLicenseType 规整许可证类型：兼容中英文标签，空/未知值回退为商业授权。
func NormalizeLicenseType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case LicenseTypeCommercial, "商业授权", "商业", "commercial license":
		return LicenseTypeCommercial
	case LicenseTypeOpenSource, "开源", "开源软件", "open source":
		return LicenseTypeOpenSource
	case LicenseTypeSubscription, "订阅", "订阅制", "subscription license":
		return LicenseTypeSubscription
	case LicenseTypeOther, "其他", "其它":
		return LicenseTypeOther
	}
	return LicenseTypeCommercial
}

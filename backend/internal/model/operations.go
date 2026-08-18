package model

import (
	"time"
)

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

// BackgroundTask records scheduler and manually retried executions.
type BackgroundTask struct {
	ID             uint                 `json:"id" gorm:"primaryKey"`
	Kind           string               `json:"kind" gorm:"size:64;not null;index"`
	Source         string               `json:"source" gorm:"size:32;not null;default:schedule;index"`
	UniqueKey      string               `json:"uniqueKey" gorm:"size:191;uniqueIndex"`
	Status         BackgroundTaskStatus `json:"status" gorm:"size:32;not null;index"`
	Payload        string               `json:"-" gorm:"type:text"`
	Result         string               `json:"result,omitempty" gorm:"type:text"`
	Error          string               `json:"error,omitempty" gorm:"type:text"`
	Attempts       int                  `json:"attempts" gorm:"not null;default:0"`
	MaxAttempts    int                  `json:"maxAttempts" gorm:"not null;default:3"`
	ScheduledAt    time.Time            `json:"scheduledAt" gorm:"not null;index"`
	StartedAt      *time.Time           `json:"startedAt"`
	FinishedAt     *time.Time           `json:"finishedAt"`
	AcknowledgedAt *time.Time           `json:"acknowledgedAt"`
	AcknowledgedBy *uint                `json:"acknowledgedBy" gorm:"index"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
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

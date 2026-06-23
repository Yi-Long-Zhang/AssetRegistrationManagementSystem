package httpapi

import (
	"time"

	"asset-registration-management-system/backend/internal/model"
)

type userRequest struct {
	Username    string     `json:"username" binding:"required"`
	Name        string     `json:"name" binding:"required"`
	DisplayName string     `json:"displayName"`
	Email       string     `json:"email"`
	Department  string     `json:"department"`
	Role        model.Role `json:"role" binding:"required"`
	Status      string     `json:"status"`
	AuthSource  string     `json:"authSource"`
	Password    string     `json:"password"`
}

type assetRequest struct {
	AssetNo    string            `json:"assetNo" binding:"required"`
	Hostname   string            `json:"hostname" binding:"required"`
	IP         string            `json:"ip" binding:"required"`
	Location   string            `json:"location"`
	Rack       string            `json:"rack"`
	OS         string            `json:"os"`
	CPU        string            `json:"cpu"`
	Memory     string            `json:"memory"`
	Disk       string            `json:"disk"`
	Owner      string            `json:"owner"`
	Status     model.AssetStatus `json:"status" binding:"required"`
	OnlineDate *time.Time        `json:"onlineDate"`
	Remark     string            `json:"remark"`
}

func (r assetRequest) toModel() model.Asset {
	return model.Asset{
		AssetNo:    r.AssetNo,
		Hostname:   r.Hostname,
		IP:         r.IP,
		Location:   r.Location,
		Rack:       r.Rack,
		OS:         r.OS,
		CPU:        r.CPU,
		Memory:     r.Memory,
		Disk:       r.Disk,
		Owner:      r.Owner,
		Status:     r.Status,
		OnlineDate: r.OnlineDate,
		Remark:     r.Remark,
	}
}

type ticketRequest struct {
	Type        model.TicketType `json:"type" binding:"required"`
	Title       string           `json:"title" binding:"required"`
	AssetID     *uint            `json:"assetId"`
	Priority    model.Priority   `json:"priority"`
	Description string           `json:"description"`
}

type ticketTypeApproverRequest struct {
	ApproverID uint `json:"approverId" binding:"required"`
}

type ticketCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

type adConfigRequest struct {
	Enabled          bool   `json:"enabled"`
	LDAPURL          string `json:"ldapUrl" binding:"required"`
	BaseDN           string `json:"baseDn" binding:"required"`
	BindDN           string `json:"bindDn" binding:"required"`
	BindPassword     string `json:"bindPassword"`
	LoginAttribute   string `json:"loginAttribute"`
	FilterUserObject bool   `json:"filterUserObject"`
	ExcludeDisabled  bool   `json:"excludeDisabled"`
	AdvancedFilter   bool   `json:"advancedFilter"`
	UserFilter       string `json:"userFilter"`
}

type adLookupRequest struct {
	Username string `json:"username" binding:"required"`
}

type adImportRequest struct {
	Username string     `json:"username" binding:"required"`
	Role     model.Role `json:"role"`
	Status   string     `json:"status"`
}

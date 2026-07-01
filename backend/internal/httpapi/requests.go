package httpapi

import (
	"time"

	"asset-registration-management-system/backend/internal/model"
)

type errorResponse struct {
	Error string `json:"error"`
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

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
	AssetNo            string            `json:"assetNo"`
	SequenceNo         string            `json:"sequenceNo"`
	AssetType          string            `json:"assetType"`
	Hostname           string            `json:"hostname"`
	IP                 string            `json:"ip" binding:"required"`
	MACAddress         string            `json:"macAddress"`
	ManagementIP       string            `json:"managementIp"`
	SerialNo           string            `json:"serialNo"`
	Manufacturer       string            `json:"manufacturer"`
	Model              string            `json:"model"`
	Location           string            `json:"location"`
	Rack               string            `json:"rack"`
	RackPosition       string            `json:"rackPosition"`
	OS                 string            `json:"os"`
	OSVersion          string            `json:"osVersion"`
	CPU                string            `json:"cpu"`
	Memory             string            `json:"memory"`
	Disk               string            `json:"disk"`
	OpenPorts          string            `json:"openPorts"`
	RunningServices    string            `json:"runningServices"`
	AppVersion         string            `json:"appVersion"`
	Subnet             string            `json:"subnet"`
	BusinessSystem     string            `json:"businessSystem"`
	Environment        string            `json:"environment"`
	Department         string            `json:"department"`
	Owner              string            `json:"owner"`
	MaintenanceVendor  string            `json:"maintenanceVendor"`
	PurchaseDate       *time.Time        `json:"purchaseDate"`
	WarrantyExpireDate *time.Time        `json:"warrantyExpireDate"`
	Status             model.AssetStatus `json:"status"`
	OnlineDate         *time.Time        `json:"onlineDate"`
	Remark             string            `json:"remark"`
}

func (r assetRequest) toModel() model.Asset {
	assetNo := r.AssetNo
	if assetNo == "" {
		assetNo = assetNoFromIP(r.IP)
	}
	status := r.Status
	if status == "" {
		status = model.AssetStatusInUse
	}
	return model.Asset{
		AssetNo:            assetNo,
		SequenceNo:         r.SequenceNo,
		AssetType:          r.AssetType,
		Hostname:           r.Hostname,
		IP:                 r.IP,
		MACAddress:         r.MACAddress,
		ManagementIP:       r.ManagementIP,
		SerialNo:           r.SerialNo,
		Manufacturer:       r.Manufacturer,
		Model:              r.Model,
		Location:           r.Location,
		Rack:               r.Rack,
		RackPosition:       r.RackPosition,
		OS:                 r.OS,
		OSVersion:          r.OSVersion,
		CPU:                r.CPU,
		Memory:             r.Memory,
		Disk:               r.Disk,
		OpenPorts:          r.OpenPorts,
		RunningServices:    r.RunningServices,
		AppVersion:         r.AppVersion,
		Subnet:             r.Subnet,
		BusinessSystem:     r.BusinessSystem,
		Environment:        r.Environment,
		Department:         r.Department,
		Owner:              r.Owner,
		MaintenanceVendor:  r.MaintenanceVendor,
		PurchaseDate:       r.PurchaseDate,
		WarrantyExpireDate: r.WarrantyExpireDate,
		Status:             status,
		OnlineDate:         r.OnlineDate,
		Remark:             r.Remark,
	}
}

type ticketRequest struct {
	Type            model.TicketType `json:"type" binding:"required"`
	Title           string           `json:"title" binding:"required"`
	AssetID         *uint            `json:"assetId"`
	Priority        model.Priority   `json:"priority"`
	Description     string           `json:"description"`
	DeviceType      string           `json:"deviceType"`
	DeviceName      string           `json:"deviceName"`
	IPAddress       string           `json:"ipAddress"`
	OpenPorts       string           `json:"openPorts"`
	RunningServices string           `json:"runningServices"`
	AppVersion      string           `json:"appVersion"`
	Manufacturer    string           `json:"manufacturer"`
	Antivirus       string           `json:"antivirus"`
	ChangeContent   string           `json:"changeContent"`
	Impact          string           `json:"impact"`
	Remark          string           `json:"remark"`
}

type ticketTypeApproverRequest struct {
	ApproverID uint `json:"approverId" binding:"required"`
}

type workflowRequest struct {
	Name    string                `json:"name"`
	Enabled bool                  `json:"enabled"`
	Nodes   []workflowNodeRequest `json:"nodes" binding:"required"`
}

type workflowNodeRequest struct {
	Name        string `json:"name" binding:"required"`
	ApproverIDs []uint `json:"approverIds" binding:"required"`
}

type ticketCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

type ticketArchiveBatchRequest struct {
	IDs []uint `json:"ids" binding:"required"`
}

type ticketActionRequest struct {
	Remark           string `json:"remark"`
	Result           string `json:"result"`
	AcceptanceResult string `json:"acceptanceResult"`
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

type mailConfigRequest struct {
	Enabled     bool   `json:"enabled"`
	SMTPHost    string `json:"smtpHost"`
	SMTPPort    int    `json:"smtpPort"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	FromAddress string `json:"fromAddress"`
	FromName    string `json:"fromName"`
	UseTLS      bool   `json:"useTls"`
	StartTLS    bool   `json:"startTls"`
}

type mailTestRequest struct {
	Recipient string `json:"recipient"`
}

type adLookupRequest struct {
	Username string `json:"username" binding:"required"`
}

type adImportRequest struct {
	Username string     `json:"username" binding:"required"`
	Role     model.Role `json:"role"`
	Status   string     `json:"status"`
}

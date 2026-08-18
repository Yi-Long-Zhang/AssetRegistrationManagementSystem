package httpapi

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) applyTodoFilter(db *gorm.DB, user model.User) *gorm.DB {
	switch user.Role {
	case model.RoleAdmin:
		return db.Where("status IN ?", []model.TicketStatus{
			model.TicketStatusPendingApproval,
			model.TicketStatusApproved,
			model.TicketStatusInProgress,
			model.TicketStatusPendingAcceptance,
			model.TicketStatusRejected,
		})
	case model.RoleApprover:
		return db.Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.ticket_id = tickets.id AND ticket_workflow_steps.id = tickets.current_workflow_step_id").
			Joins("JOIN ticket_workflow_step_approvers ON ticket_workflow_step_approvers.step_id = ticket_workflow_steps.id").
			Where("tickets.status = ? AND ticket_workflow_step_approvers.user_id IN ?", model.TicketStatusPendingApproval, h.approverIDsWithProxy(user.ID))
	case model.RoleAssetManager:
		return db.Where("status IN ?", []model.TicketStatus{model.TicketStatusApproved, model.TicketStatusInProgress})
	default:
		return db.Where("status IN ? AND applicant_id = ?", []model.TicketStatus{model.TicketStatusPendingAcceptance, model.TicketStatusRejected}, user.ID)
	}
}

// approverIDsWithProxy 返回用户的审批范围：自己 + 所有设置了本人为代理审批人的用户。
// 用于待办筛选与审批权限判断（单级代理，不支持代理链）。
func (h *Handler) approverIDsWithProxy(userID uint) []uint {
	var proxied []uint
	_ = h.db.Model(&model.User{}).Where("proxy_user_id = ?", userID).Pluck("id", &proxied).Error
	ids := make([]uint, 0, len(proxied)+1)
	ids = append(ids, userID)
	ids = append(ids, proxied...)
	return ids
}

func (h *Handler) defaultApproverID(c *gin.Context, ticketType model.TicketType) (uint, bool) {
	var approverConfig model.TicketTypeApprover
	if err := h.db.Where("type = ?", ticketType).First(&approverConfig).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorJSON(c, http.StatusBadRequest, "该工单类型尚未配置默认审批人")
			return 0, false
		}
		errorJSON(c, http.StatusInternalServerError, "查询默认审批人失败")
		return 0, false
	}
	return approverConfig.ApproverID, true
}

func (h *Handler) collaborationTicket(c *gin.Context) (model.Ticket, bool) {
	id, ok := parseID(c)
	if !ok {
		return model.Ticket{}, false
	}
	var ticket model.Ticket
	if err := h.db.First(&ticket, id).Error; err != nil {
		statusForDBError(c, err, "工单不存在")
		return model.Ticket{}, false
	}
	if !h.canCollaborate(c, ticket) {
		return model.Ticket{}, false
	}
	return ticket, true
}

func (h *Handler) canCollaborate(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ApproverID != nil && *ticket.ApproverID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	if user.Role == model.RoleAssetManager {
		return ticket.Status == model.TicketStatusApproved ||
			ticket.Status == model.TicketStatusInProgress ||
			ticket.Status == model.TicketStatusPendingAcceptance
	}
	errorJSON(c, http.StatusForbidden, "没有权限访问该工单协作内容")
	return false
}

func (h *Handler) canDownloadArchive(c *gin.Context, ticket model.Ticket) bool {
	user := currentUser(c)
	if user.Role == model.RoleAdmin || ticket.ApplicantID == user.ID {
		return true
	}
	if ticket.ExecutorID != nil && *ticket.ExecutorID == user.ID {
		return true
	}
	var count int64
	_ = h.db.Model(&model.TicketWorkflowStepApprover{}).
		Joins("JOIN ticket_workflow_steps ON ticket_workflow_steps.id = ticket_workflow_step_approvers.step_id").
		Where("ticket_workflow_steps.ticket_id = ? AND ticket_workflow_step_approvers.user_id = ?", ticket.ID, user.ID).
		Count(&count).Error
	if count > 0 {
		return true
	}
	errorJSON(c, http.StatusForbidden, "没有权限下载该工单归档")
	return false
}

func applyTicketRequest(ticket *model.Ticket, req ticketRequest) {
	ticket.Type = req.Type
	ticket.Title = req.Title
	ticket.AssetID = req.AssetID
	ticket.Priority = defaultPriority(req.Priority)
	ticket.Description = req.Description
	ticket.DeviceType = req.DeviceType
	ticket.DeviceName = req.DeviceName
	ticket.IPAddress = req.IPAddress
	ticket.OpenPorts = req.OpenPorts
	ticket.RunningServices = req.RunningServices
	ticket.AppVersion = req.AppVersion
	ticket.Manufacturer = req.Manufacturer
	ticket.Antivirus = req.Antivirus
	ticket.ChangeContent = req.ChangeContent
	ticket.Impact = req.Impact
	ticket.Remark = req.Remark
}

func (h *Handler) closeTicketWithArchive(c *gin.Context, ticket *model.Ticket) (string, string, bool) {
	var full model.Ticket
	if err := h.db.Preload("Applicant").Preload("Asset").Preload("Assets.Asset").Preload("Executor").Preload("Records.Actor").
		Preload("WorkflowSteps.Actor").Preload("WorkflowSteps", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order asc") }).
		First(&full, ticket.ID).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "加载归档数据失败")
		return "", "", false
	}
	full.Status = ticket.Status
	full.Result = ticket.Result
	full.AcceptanceResult = ticket.AcceptanceResult
	archiveNo, archivePath, err := h.archiver.Generate(c.Request.Context(), service.TicketArchiveData{Ticket: full}, h.cfg.Storage.TicketTemplatePath, h.cfg.Storage.TicketArchiveDir, h.cfg.Storage.LibreOfficeBin)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "生成归档 PDF 失败: "+err.Error())
		return "", "", false
	}
	if err := h.writeBackTicketAsset(ticket); err != nil {
		_ = os.Remove(archivePath)
		errorJSON(c, http.StatusBadRequest, "写回资产台账失败: "+err.Error())
		return "", "", false
	}
	return archiveNo, archivePath, true
}

func (h *Handler) writeBackTicketAsset(ticket *model.Ticket) error {
	// 多资产：优先取关联表；空则回退到单资产 AssetID（兼容旧工单）
	assetIDs := []uint{}
	var links []model.TicketAsset
	if err := h.db.Where("ticket_id = ?", ticket.ID).Find(&links).Error; err != nil {
		return err
	}
	for _, link := range links {
		assetIDs = append(assetIDs, link.AssetID)
	}
	if len(assetIDs) == 0 && ticket.AssetID != nil {
		assetIDs = []uint{*ticket.AssetID}
	}
	if len(assetIDs) == 0 {
		return nil
	}
	for _, assetID := range assetIDs {
		var asset model.Asset
		if err := h.db.First(&asset, assetID).Error; err != nil {
			return err
		}
		if ticket.DeviceType != "" {
			asset.AssetType = ticket.DeviceType
		}
		if ticket.DeviceName != "" {
			asset.Hostname = ticket.DeviceName
		}
		if ticket.IPAddress != "" {
			asset.IP = ticket.IPAddress
		}
		if ticket.OpenPorts != "" {
			asset.OpenPorts = ticket.OpenPorts
		}
		if ticket.RunningServices != "" {
			asset.RunningServices = ticket.RunningServices
		}
		if ticket.AppVersion != "" {
			asset.AppVersion = ticket.AppVersion
		}
		if ticket.Manufacturer != "" {
			asset.Manufacturer = ticket.Manufacturer
		}
		if err := h.db.Save(&asset).Error; err != nil {
			return err
		}
		actorID := uint(0)
		if err := (&service.AssetSnapshotter{DB: h.db}).CreateSnapshot(&asset, model.SnapshotSourceTicket, &actorID, "update"); err != nil {
			return err
		}
	}
	return nil
}

func uniqueStoredName(original string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(original))
	}
	return fmt.Sprintf("%d-%s-%s", time.Now().UnixNano(), hex.EncodeToString(random[:]), filepath.Base(original))
}

// derefStr 解引用字符串指针，空指针返回空串。
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func addFileToZip(zipWriter *zip.Writer, sourcePath, archiveName string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	writer, err := zipWriter.Create(archiveName)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, source)
	return err
}

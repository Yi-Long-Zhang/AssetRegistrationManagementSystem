package httpapi

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"asset-registration-management-system/backend/internal/model"
	"asset-registration-management-system/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type discoveryRuleRequest struct {
	Name            string `json:"name" binding:"required"`
	Targets         string `json:"targets" binding:"required"`
	Ports           string `json:"ports"`
	ProbePorts      string `json:"probePorts"`
	ServiceDetect   bool   `json:"serviceDetect"`
	IntervalMinutes int    `json:"intervalMinutes"`
	AutoAdopt       bool   `json:"autoAdopt"`
	AutoApply       bool   `json:"autoApply"`
	Enabled         bool   `json:"enabled"`
}

func (r discoveryRuleRequest) toModel(id uint) model.DiscoveryRule {
	if r.IntervalMinutes <= 0 {
		r.IntervalMinutes = 60
	}
	return model.DiscoveryRule{
		ID:              id,
		Name:            r.Name,
		Targets:         r.Targets,
		Ports:           r.Ports,
		ProbePorts:      r.ProbePorts,
		ServiceDetect:   r.ServiceDetect,
		IntervalMinutes: r.IntervalMinutes,
		AutoAdopt:       r.AutoAdopt,
		AutoApply:       r.AutoApply,
		Enabled:         r.Enabled,
	}
}

type discoveryHostActionRequest struct {
	HostIDs []uint `json:"hostIds" binding:"required"`
}

// ListDiscoveryRules 发现规则列表
func (h *Handler) ListDiscoveryRules(c *gin.Context) {
	var rules []model.DiscoveryRule
	if err := h.db.Order("id desc").Find(&rules).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "加载发现规则失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, rules)
}

// CreateDiscoveryRule 创建发现规则
func (h *Handler) CreateDiscoveryRule(c *gin.Context) {
	var req discoveryRuleRequest
	if !bind(c, &req) {
		return
	}
	if err := service.ValidateTargets(req.Targets); err != nil {
		errorJSON(c, http.StatusBadRequest, "目标格式错误: "+err.Error())
		return
	}
	if err := service.ValidatePorts(req.Ports); err != nil {
		errorJSON(c, http.StatusBadRequest, "端口格式错误: "+err.Error())
		return
	}
	rule := req.toModel(0)
	if err := h.db.Create(&rule).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "创建发现规则失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_rule", rule.ID, "create", rule.Name)
	c.JSON(http.StatusCreated, rule)
}

// UpdateDiscoveryRule 更新发现规则
func (h *Handler) UpdateDiscoveryRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var existing model.DiscoveryRule
	if !h.findByID(c, id, &existing) {
		return
	}
	var req discoveryRuleRequest
	if !bind(c, &req) {
		return
	}
	if err := service.ValidateTargets(req.Targets); err != nil {
		errorJSON(c, http.StatusBadRequest, "目标格式错误: "+err.Error())
		return
	}
	if err := service.ValidatePorts(req.Ports); err != nil {
		errorJSON(c, http.StatusBadRequest, "端口格式错误: "+err.Error())
		return
	}
	updated := req.toModel(id)
	updated.CreatedAt = existing.CreatedAt
	if err := h.db.Save(&updated).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "更新发现规则失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_rule", updated.ID, "update", updated.Name)
	c.JSON(http.StatusOK, updated)
}

// DeleteDiscoveryRule 删除发现规则
func (h *Handler) DeleteDiscoveryRule(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var rule model.DiscoveryRule
	if !h.findByID(c, id, &rule) {
		return
	}
	if err := h.db.Delete(&rule).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "删除发现规则失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_rule", rule.ID, "delete", rule.Name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// StartDiscoveryRun 手动触发发现：创建运行记录并异步执行（返回 202）
func (h *Handler) StartDiscoveryRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var rule model.DiscoveryRule
	if !h.findByID(c, id, &rule) {
		return
	}
	var running int64
	h.db.Model(&model.DiscoveryRun{}).
		Where("rule_id = ? AND status = ?", rule.ID, model.DiscoveryRunStatusRunning).
		Count(&running)
	if running > 0 {
		errorJSON(c, http.StatusConflict, "该规则已有正在执行的发现任务")
		return
	}
	svc := service.NewDiscoveryService(h.db, h.cfg)
	run, err := svc.StartRun(rule.ID, "manual")
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "启动发现任务失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_run", run.ID, "start", rule.Name)
	go func(runID uint) {
		ctx, cancel := context.WithTimeout(context.Background(),
			time.Duration(h.cfg.Discovery.ScanTimeoutSec)*time.Second+30*time.Second)
		defer cancel()
		if err := svc.ExecuteRun(ctx, runID); err != nil {
			log.Printf("discovery run %d failed: %v", runID, err)
		}
	}(run.ID)
	c.JSON(http.StatusAccepted, run)
}

// TestDiscoveryRun 试跑规则并报告 nmap 可用性与扫描摘要
func (h *Handler) TestDiscoveryRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var rule model.DiscoveryRule
	if !h.findByID(c, id, &rule) {
		return
	}
	svc := service.NewDiscoveryService(h.db, h.cfg)
	bin, err := svc.Bin()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"ok": false, "nmapBin": "", "error": err.Error(),
			"hint": "请运行 scripts/setup-nmap.ps1（Windows）或 scripts/setup-nmap.sh（Linux/macOS）安装 nmap，或在配置中设置 discovery.nmap_bin",
		})
		return
	}
	cfg := h.cfg
	cfg.Discovery.ScanTimeoutSec = 20 // 试跑使用较短超时
	results, err := service.Scan(c.Request.Context(), svc.Runner, bin, rule, cfg.Discovery)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "nmapBin": bin, "error": err.Error()})
		return
	}
	up := 0
	for _, r := range results {
		if r.Status == "up" {
			up++
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "nmapBin": bin, "total": len(results), "up": up})
}

// ListDiscoveryRuns 运行记录列表（支持 ruleId/status 过滤与分页）
func (h *Handler) ListDiscoveryRuns(c *gin.Context) {
	query := h.db.Model(&model.DiscoveryRun{})
	if value := strings.TrimSpace(c.Query("ruleId")); value != "" {
		if id, err := strconv.Atoi(value); err == nil {
			query = query.Where("rule_id = ?", id)
		}
	}
	if value := strings.TrimSpace(c.Query("status")); value != "" {
		query = query.Where("status = ?", value)
	}
	var total int64
	query.Count(&total)
	page, pageSize := assetPagination(c)
	var runs []model.DiscoveryRun
	if err := query.Preload("Rule").Order("id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&runs).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "加载运行记录失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": runs, "total": total})
}

// GetDiscoveryRun 运行记录详情（含主机结果与匹配资产）
func (h *Handler) GetDiscoveryRun(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var run model.DiscoveryRun
	if err := h.db.Preload("Rule").Preload("Hosts.MatchedAsset").First(&run, id).Error; err != nil {
		statusForDBError(c, err, "运行记录不存在")
		return
	}
	c.JSON(http.StatusOK, run)
}

// AdoptDiscoveryHosts 将运行结果中选中的新主机纳管为资产
func (h *Handler) AdoptDiscoveryHosts(c *gin.Context) {
	runID, ok := parseID(c)
	if !ok {
		return
	}
	var req discoveryHostActionRequest
	if !bind(c, &req) {
		return
	}
	svc := service.NewDiscoveryService(h.db, h.cfg)
	count, err := svc.AdoptHosts(c.Request.Context(), runID, req.HostIDs, currentUser(c).ID)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "纳管失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_run", runID, "adopt", strconv.Itoa(count)+" 台主机")
	c.JSON(http.StatusOK, gin.H{"adopted": count})
}

// ApplyDiscoveryHosts 将运行结果中选中的变更应用到资产台账
func (h *Handler) ApplyDiscoveryHosts(c *gin.Context) {
	runID, ok := parseID(c)
	if !ok {
		return
	}
	var req discoveryHostActionRequest
	if !bind(c, &req) {
		return
	}
	svc := service.NewDiscoveryService(h.db, h.cfg)
	count, err := svc.ApplyHostChanges(c.Request.Context(), runID, req.HostIDs, currentUser(c).ID)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "应用变更失败: "+err.Error())
		return
	}
	h.audit(currentUser(c).ID, "discovery_run", runID, "apply", strconv.Itoa(count)+" 条变更")
	c.JSON(http.StatusOK, gin.H{"applied": count})
}

// ListAssetHistory 资产变更历史（快照时间线，倒序）
func (h *Handler) ListAssetHistory(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var asset model.Asset
	if !h.findByID(c, id, &asset) {
		return
	}
	var snaps []model.AssetSnapshot
	if err := h.db.Where("asset_id = ?", id).Order("id desc").Limit(200).Find(&snaps).Error; err != nil {
		errorJSON(c, http.StatusBadRequest, "加载资产历史失败: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, snaps)
}

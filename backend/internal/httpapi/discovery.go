package httpapi

import (
	"context"
	"log"
	"net/http"
	"sort"
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
	AutoTicket      bool   `json:"autoTicket"`
	Incremental     bool   `json:"incremental"`
	ScanWindowStart string `json:"scanWindowStart"`
	ScanWindowEnd   string `json:"scanWindowEnd"`
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
		AutoTicket:      r.AutoTicket,
		Incremental:     r.Incremental,
		ScanWindowStart: r.ScanWindowStart,
		ScanWindowEnd:   r.ScanWindowEnd,
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

// discoveryTrendItem 某一天的发现统计
type discoveryTrendItem struct {
	Date    string `json:"date"`    // YYYY-MM-DD
	Runs    int    `json:"runs"`    // 成功运行次数
	New     int    `json:"new"`     // 新增主机数
	Changed int    `json:"changed"` // 变更主机数
	Offline int    `json:"offline"` // 离线主机数
	Online  int    `json:"online"`  // 恢复在线主机数
}

// GetDiscoveryTrend 发现趋势统计：按天聚合成功运行的 new/changed/offline/online 计数。
// @Summary 发现趋势统计
// @Description 按天聚合各发现运行的变更统计（默认最近 14 天，可用 days 参数调整）
// @Tags discovery
// @Produce json
// @Param days query int false "统计天数（默认 14）"
// @Success 200 {array} discoveryTrendItem
// @Failure 500 {object} map[string]string
// @Router /discovery/stats/trend [get]
// @Security BearerAuth
func (h *Handler) GetDiscoveryTrend(c *gin.Context) {
	days := 14
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	since := time.Now().AddDate(0, 0, -days)
	var runs []model.DiscoveryRun
	if err := h.db.Where("status = ? AND started_at >= ?", model.DiscoveryRunStatusSuccess, since).
		Find(&runs).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询发现趋势失败: "+err.Error())
		return
	}
	byDate := map[string]*discoveryTrendItem{}
	for _, r := range runs {
		date := r.StartedAt.Format("2006-01-02")
		item := byDate[date]
		if item == nil {
			item = &discoveryTrendItem{Date: date}
			byDate[date] = item
		}
		item.Runs++
		item.New += r.NewCount
		item.Changed += r.ChangedCount
		item.Offline += r.OfflineCount
		item.Online += r.OnlineCount
	}
	// 补齐缺失日期，保证连续
	var out []discoveryTrendItem
	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if item, ok := byDate[date]; ok {
			out = append(out, *item)
		} else {
			out = append(out, discoveryTrendItem{Date: date})
		}
	}
	c.JSON(http.StatusOK, out)
}

// subnetStat 一个网段的资产数量
type subnetStat struct {
	Subnet string `json:"subnet"`
	Count  int    `json:"count"`
}

// GetDiscoverySubnetStats 资产网段分布统计：按资产 subnet 字段聚合，未填写的按 IP 前两段兜底。
// @Summary 资产网段分布
// @Description 按网段聚合资产数量（subnet 字段优先，未填时按 IP /16 兜底）
// @Tags discovery
// @Produce json
// @Success 200 {array} subnetStat
// @Failure 500 {object} map[string]string
// @Router /discovery/stats/subnets [get]
// @Security BearerAuth
func (h *Handler) GetDiscoverySubnetStats(c *gin.Context) {
	var assets []model.Asset
	if err := h.db.Select("subnet", "ip").Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询网段分布失败: "+err.Error())
		return
	}
	counts := map[string]int{}
	for _, a := range assets {
		subnet := strings.TrimSpace(a.Subnet)
		if subnet == "" {
			subnet = subnetOfIP(a.IP)
		}
		if subnet != "" {
			counts[subnet]++
		}
	}
	var out []subnetStat
	for subnet, count := range counts {
		out = append(out, subnetStat{Subnet: subnet, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	c.JSON(http.StatusOK, out)
}

// subnetOfIP 取 IPv4 的 /16 网段（如 192.168.1.10 -> 192.168.0.0/16）。
func subnetOfIP(ip string) string {
	parts := strings.Split(strings.TrimSpace(ip), ".")
	if len(parts) != 4 {
		return ""
	}
	return parts[0] + "." + parts[1] + ".0.0/16"
}

// serviceStat 一个端口/服务的资产覆盖数
type serviceStat struct {
	Port    string `json:"port"`
	Service string `json:"service"`
	Count   int    `json:"count"`
}

// GetDiscoveryServiceStats 端口/服务矩阵统计：统计各开放端口被多少台资产使用（含服务名）。
// @Summary 端口/服务矩阵
// @Description 按开放端口聚合资产数量（含 -sV 识别的服务名），按数量降序返回
// @Tags discovery
// @Produce json
// @Param limit query int false "返回条数（默认 50）"
// @Success 200 {array} serviceStat
// @Failure 500 {object} map[string]string
// @Router /discovery/stats/services [get]
// @Security BearerAuth
func (h *Handler) GetDiscoveryServiceStats(c *gin.Context) {
	limit := 50
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	var assets []model.Asset
	if err := h.db.Select("open_ports", "running_services").Find(&assets).Error; err != nil {
		errorJSON(c, http.StatusInternalServerError, "查询端口矩阵失败: "+err.Error())
		return
	}
	counts := map[string]*serviceStat{}
	for _, a := range assets {
		// 服务名映射：running_services 行 "80/tcp: http nginx 1.18" → port -> service
		svcByPort := map[string]string{}
		for _, line := range strings.Split(a.RunningServices, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			idx := strings.Index(line, ":")
			if idx <= 0 {
				continue
			}
			port := strings.Split(strings.TrimSpace(line[:idx]), "/")[0]
			desc := strings.TrimSpace(line[idx+1:])
			// 取服务名（第一个词）
			if desc != "" {
				svcByPort[port] = strings.Fields(desc)[0]
			}
		}
		for _, p := range strings.Split(a.OpenPorts, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			port := strings.Split(p, "/")[0]
			key := port
			stat := counts[key]
			if stat == nil {
				stat = &serviceStat{Port: port, Service: svcByPort[port]}
				counts[key] = stat
			}
			stat.Count++
			if stat.Service == "" {
				stat.Service = svcByPort[port]
			}
		}
	}
	var out []serviceStat
	for _, stat := range counts {
		out = append(out, *stat)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Port < out[j].Port
	})
	if len(out) > limit {
		out = out[:limit]
	}
	c.JSON(http.StatusOK, out)
}

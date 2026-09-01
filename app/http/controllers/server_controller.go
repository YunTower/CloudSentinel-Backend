package controllers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"goravel/app/cryptoutil"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/services/websocket/panelkey"
	"goravel/app/utils"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
)

// GetPanelFingerprint 返回安装 Agent 时需要人工核对的面板公钥指纹。
// 仅管理员会话可读取，避免把“面板提供”的人工校验材料混入未认证引导通道。
func (c *ServerController) GetPanelFingerprint(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	_, publicKey, err := panelkey.GetOrGenerate()
	if err != nil {
		facades.Log().Errorf("获取面板公钥失败: %v", err)
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "获取面板公钥指纹失败")
	}
	fingerprint, err := cryptoutil.GetPublicKeyFingerprint(publicKey)
	if err != nil {
		facades.Log().Errorf("计算面板公钥指纹失败: %v", err)
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "计算面板公钥指纹失败")
	}
	return utils.SuccessResponse(ctx, "success", map[string]string{"panel_fingerprint": fingerprint})
}

// parseExpireTime 解析到期时间字符串
func parseExpireTime(s string) *time.Time {
	layouts := []string{
		time.RFC3339Nano, // 2026-02-28T15:41:17.000Z
		time.RFC3339,     // 2026-02-28T15:41:17Z
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			return &parsed
		}
	}
	return nil
}

func normalizeTrafficSettings(
	trafficLimitType string,
	trafficResetCycle string,
	trafficCustomCycleDays *int,
) (string, string, *int) {
	switch trafficResetCycle {
	case "monthly", "quarterly", "yearly":
		return "periodic", trafficResetCycle, nil
	case "custom":
		return "periodic", "custom", trafficCustomCycleDays
	case "unlimited":
		return "permanent", "unlimited", nil
	}

	switch trafficLimitType {
	case "permanent", "unlimited":
		return "permanent", "unlimited", nil
	case "periodic":
		return "periodic", trafficResetCycle, trafficCustomCycleDays
	default:
		return trafficLimitType, trafficResetCycle, trafficCustomCycleDays
	}
}

func resolveTrafficCycleForResponse(trafficLimitType string, trafficResetCycle string) string {
	if trafficResetCycle != "" {
		return trafficResetCycle
	}
	if trafficLimitType == "permanent" || trafficLimitType == "unlimited" {
		return "unlimited"
	}
	return ""
}

func maskAgentKey(agentKey string) string {
	if len(agentKey) <= 8 {
		return "********"
	}
	return agentKey[:4] + "********" + agentKey[len(agentKey)-4:]
}

type ServerController struct{}

func NewServerController() *ServerController {
	return &ServerController{}
}

// CreateServer 创建服务器
func (c *ServerController) CreateServer(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	type BillingRequest struct {
		BillingCycle           string   `json:"billing_cycle" form:"billing_cycle"`
		CustomCycleDays        *int     `json:"custom_cycle_days" form:"custom_cycle_days"`
		Price                  *float64 `json:"price" form:"price"`
		ExpireTime             *string  `json:"expire_time" form:"expire_time"`
		BandwidthMbps          int      `json:"bandwidth_mbps" form:"bandwidth_mbps"`
		TrafficLimitType       string   `json:"traffic_limit_type" form:"traffic_limit_type"`
		TrafficLimitBytes      int64    `json:"traffic_limit_bytes" form:"traffic_limit_bytes"`
		TrafficResetCycle      string   `json:"traffic_reset_cycle" form:"traffic_reset_cycle"`
		TrafficCustomCycleDays *int     `json:"traffic_custom_cycle_days" form:"traffic_custom_cycle_days"`
		ShowBillingCycle       *bool    `json:"show_billing_cycle" form:"show_billing_cycle"`
	}

	type NetworkRequest struct {
		ShowTrafficLimit      *bool `json:"show_traffic_limit" form:"show_traffic_limit"`
		ShowTrafficResetCycle *bool `json:"show_traffic_reset_cycle" form:"show_traffic_reset_cycle"`
	}

	type CreateServerRequest struct {
		Name     string          `json:"name" form:"name"`
		IP       string          `json:"ip" form:"ip"`
		Location string          `json:"location" form:"location"`
		OS       string          `json:"os" form:"os"`
		GroupID  *uint           `json:"group_id" form:"group_id"`
		Billing  *BillingRequest `json:"billing" form:"billing"`
		Network  *NetworkRequest `json:"network" form:"network"`
	}

	var req CreateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusBadRequest, "请求参数错误", err)
	}

	// 验证必填字段
	if req.Name == "" || req.IP == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "名称和IP地址为必填项")
	}

	// 初始化 billing
	if req.Billing == nil {
		req.Billing = &BillingRequest{}
	}

	// 初始化 network
	if req.Network == nil {
		req.Network = &NetworkRequest{}
	}

	// 生成UUID作为server_id
	serverID := uuid.New().String()

	// 生成agent_key
	agentKey := uuid.New().String()

	// 解析到期时间
	var expireTime *time.Time
	if req.Billing.ExpireTime != nil && *req.Billing.ExpireTime != "" {
		expireTime = parseExpireTime(*req.Billing.ExpireTime)
	}

	// 处理显示开关字段
	showBillingCycle := false
	if req.Billing.ShowBillingCycle != nil {
		showBillingCycle = *req.Billing.ShowBillingCycle
	}

	showTrafficLimit := false
	if req.Network.ShowTrafficLimit != nil {
		showTrafficLimit = *req.Network.ShowTrafficLimit
	}

	showTrafficResetCycle := false
	if req.Network.ShowTrafficResetCycle != nil {
		showTrafficResetCycle = *req.Network.ShowTrafficResetCycle
	}

	normalizedTrafficLimitType, normalizedTrafficResetCycle, normalizedTrafficCustomCycleDays :=
		normalizeTrafficSettings(
			req.Billing.TrafficLimitType,
			req.Billing.TrafficResetCycle,
			req.Billing.TrafficCustomCycleDays,
		)

	// 创建服务器模型
	server := &models.Server{
		ID:                     serverID,
		Name:                   req.Name,
		IP:                     req.IP,
		Location:               req.Location,
		Status:                 "offline",
		OS:                     req.OS,
		AgentKey:               agentKey,
		AgentKeyHash:           services.HashAgentKey(agentKey),
		Cores:                  1,
		GroupID:                req.GroupID,
		BillingCycle:           req.Billing.BillingCycle,
		CustomCycleDays:        req.Billing.CustomCycleDays,
		Price:                  req.Billing.Price,
		ExpireTime:             expireTime,
		BandwidthMbps:          req.Billing.BandwidthMbps,
		TrafficLimitType:       normalizedTrafficLimitType,
		TrafficLimitBytes:      req.Billing.TrafficLimitBytes,
		TrafficResetCycle:      normalizedTrafficResetCycle,
		TrafficCustomCycleDays: normalizedTrafficCustomCycleDays,
		ShowBillingCycle:       showBillingCycle,
		ShowTrafficLimit:       showTrafficLimit,
		ShowTrafficResetCycle:  showTrafficResetCycle,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	serverRepo := repositories.GetServerRepository()
	if err := serverRepo.Create(server); err != nil {
		facades.Log().Errorf("创建服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "创建服务器失败", err)
	}

	// 创建服务器后，默认开启所有已配置的全局通知渠道
	alertService := services.NewAlertService()
	notificationRepo := repositories.GetAlertNotificationRepository()
	globalNotifications, err := notificationRepo.GetAll()
	if err == nil {
		// 获取所有已配置且启用的全局通知渠道
		channels := make(map[string]bool)
		for _, notif := range globalNotifications {
			if notif.Enabled && notif.ConfigJson != "" {
				// 默认开启已配置的全局通知渠道
				channels[notif.NotificationType] = true
			}
		}
		// 保存服务器通知渠道配置
		if len(channels) > 0 {
			if err := alertService.SaveServerNotificationChannels(serverID, channels); err != nil {
				facades.Log().Warningf("创建服务器默认通知渠道配置失败: %v", err)
			}
		}
	}

	facades.Log().Infof("成功创建服务器: %s (IP: %s)", req.Name, req.IP)

	// 返回服务器信息和agent_key
	responseData := map[string]interface{}{
		"id":         server.ID,
		"name":       server.Name,
		"ip":         server.IP,
		"status":     server.Status,
		"agent_key":  server.AgentKey,
		"created_at": server.CreatedAt,
		"updated_at": server.UpdatedAt,
	}

	// 添加付费信息
	billingData := map[string]interface{}{
		"show_billing_cycle": server.ShowBillingCycle,
	}
	if server.BillingCycle != "" {
		billingData["billing_cycle"] = server.BillingCycle
	}
	if server.CustomCycleDays != nil {
		billingData["custom_cycle_days"] = *server.CustomCycleDays
	}
	if server.Price != nil {
		billingData["price"] = *server.Price
	}
	if server.ExpireTime != nil {
		billingData["expire_time"] = server.ExpireTime.Format("2006-01-02 15:04:05")
	}
	if server.BandwidthMbps > 0 {
		billingData["bandwidth_mbps"] = server.BandwidthMbps
	}
	if server.TrafficLimitType != "" {
		billingData["traffic_limit_type"] = server.TrafficLimitType
	}
	if server.TrafficLimitBytes > 0 {
		billingData["traffic_limit_bytes"] = server.TrafficLimitBytes
	}
	if resolvedTrafficCycle := resolveTrafficCycleForResponse(
		server.TrafficLimitType,
		server.TrafficResetCycle,
	); resolvedTrafficCycle != "" {
		billingData["traffic_reset_cycle"] = resolvedTrafficCycle
	}
	if server.TrafficCustomCycleDays != nil {
		billingData["traffic_custom_cycle_days"] = *server.TrafficCustomCycleDays
	}
	responseData["billing"] = billingData

	// 添加网络信息
	networkData := map[string]interface{}{
		"show_traffic_limit":       server.ShowTrafficLimit,
		"show_traffic_reset_cycle": server.ShowTrafficResetCycle,
	}
	responseData["network"] = networkData

	return utils.SuccessResponseWithStatus(ctx, http.StatusCreated, "服务器创建成功", responseData)
}

// GetServers 获取服务器列表
func (c *ServerController) GetServers(ctx http.Context) http.Response {
	// 获取用户类型
	userType, _ := ctx.Value("user_type").(string)
	if userType == "" {
		userType = "guest" // 默认为游客
	}

	// 获取分组筛选参数
	groupIDStr := ctx.Request().Query("group_id")
	var groupID *uint
	if groupIDStr != "" {
		if id, err := strconv.ParseUint(groupIDStr, 10, 32); err == nil {
			uid := uint(id)
			groupID = &uid
		}
	}

	settingRepo := repositories.GetSystemSettingRepository()
	serverRepo := repositories.GetServerRepository()
	metricRepo := repositories.GetServerMetricRepository()

	// 获取敏感信息隐藏设置
	hideSensitiveInfo := settingRepo.GetBool("hide_sensitive_info", true)

	// 判断是否需要隐藏敏感信息
	shouldHideSensitive := userType == "guest" && hideSensitiveInfo
	// 判断是否是管理员
	isAdmin := userType == "admin"
	// 公开展示数据契约始终对 guest 生效；禁用时不返回服务器数据。
	publicDisplayCfg := loadPublicDisplayConfigV1()
	applyPublicDisplay := userType == "guest"

	// 获取服务器列表（支持按分组筛选）
	var allServers []*models.Server
	var err error
	if groupID != nil {
		allServers, err = serverRepo.GetByGroupID(*groupID)
	} else {
		allServers, err = serverRepo.GetAll()
	}
	if err != nil {
		facades.Log().Errorf("获取服务器列表失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "获取服务器列表失败", err)
	}

	// 对 guest 应用公开展示策略（后端强制执行）。策略关闭时使用空列表，
	// 避免公开接口退化为返回所有服务器及其计费/流量等字段。
	if applyPublicDisplay && !publicDisplayCfg.Enabled {
		allServers = []*models.Server{}
	} else if applyPublicDisplay && publicDisplayCfg.ServerFilter.Mode == publicDisplayServerFilterModeAllowList {
		filtered := make([]*models.Server, 0, len(allServers))
		for _, s := range allServers {
			if isServerAllowedForGuestV1(publicDisplayCfg, s.ID, s.GroupID) {
				filtered = append(filtered, s)
			}
		}
		allServers = filtered
	}

	// 收集所有服务器ID
	serverIDs := make([]string, 0, len(allServers))
	for _, s := range allServers {
		serverIDs = append(serverIDs, s.ID)
	}

	// 批量获取最新指标和磁盘信息
	latestMetrics, err := metricRepo.GetLatestByServerIDs(serverIDs)
	if err != nil {
		facades.Log().Errorf("获取服务器指标失败: %v", err)
		latestMetrics = make(map[string]*models.ServerMetric)
	}

	serversWithDisks, err := serverRepo.GetWithDisks(serverIDs)
	if err != nil {
		facades.Log().Errorf("获取服务器磁盘信息失败: %v", err)
		serversWithDisks = allServers
	}

	servers := make([]map[string]interface{}, 0, len(allServers))
	for _, server := range serversWithDisks {
		serverData := map[string]interface{}{
			"id":           server.ID,
			"name":         server.Name,
			"ip":           server.IP,
			"location":     server.Location,
			"os":           server.OS,
			"architecture": server.Architecture,
			"system_name":  server.SystemName,
			"cpu_name":     server.CpuName,
			"status":       server.Status,
			"cores":        server.Cores,
			"created_at":   server.CreatedAt.Format("2006-01-02 15:04:05"),
			"updated_at":   server.UpdatedAt.Format("2006-01-02 15:04:05"),
		}

		// 添加分组和付费信息
		if server.GroupID != nil {
			serverData["group_id"] = *server.GroupID
			if server.ServerGroup != nil {
				serverData["group"] = map[string]interface{}{
					"id":          server.ServerGroup.ID,
					"name":        server.ServerGroup.Name,
					"description": server.ServerGroup.Description,
					"color":       server.ServerGroup.Color,
				}
			}
		}

		billingData := map[string]interface{}{
			"show_billing_cycle": server.ShowBillingCycle,
		}

		networkData := map[string]interface{}{
			"show_traffic_limit":       server.ShowTrafficLimit,
			"show_traffic_reset_cycle": server.ShowTrafficResetCycle,
		}
		serverData["network"] = networkData

		if server.BillingCycle != "" {
			billingData["billing_cycle"] = server.BillingCycle
		}
		if server.CustomCycleDays != nil {
			billingData["custom_cycle_days"] = *server.CustomCycleDays
		}
		if server.Price != nil {
			billingData["price"] = *server.Price
		}
		if server.ExpireTime != nil {
			billingData["expire_time"] = server.ExpireTime.Format("2006-01-02 15:04:05")
		} else {
			billingData["expire_time"] = nil
		}
		if server.BandwidthMbps > 0 {
			billingData["bandwidth_mbps"] = server.BandwidthMbps
		}
		if server.TrafficLimitType != "" {
			billingData["traffic_limit_type"] = server.TrafficLimitType
		}
		if server.TrafficLimitBytes > 0 {
			billingData["traffic_limit_bytes"] = server.TrafficLimitBytes
		}
		if resolvedTrafficCycle := resolveTrafficCycleForResponse(
			server.TrafficLimitType,
			server.TrafficResetCycle,
		); resolvedTrafficCycle != "" {
			billingData["traffic_reset_cycle"] = resolvedTrafficCycle
		}
		if server.TrafficCustomCycleDays != nil {
			billingData["traffic_custom_cycle_days"] = *server.TrafficCustomCycleDays
		}
		serverData["billing"] = billingData

		// 计算运行时间
		serverData["uptime"] = services.CalculateUptime(server.BootTime, nil)

		// 设置指标数据
		if metric, exists := latestMetrics[server.ID]; exists {
			serverData["metrics"] = map[string]interface{}{
				"cpu_usage":        services.FormatMetricValue(metric.CPUUsage),
				"memory_usage":     services.FormatMetricValue(metric.MemoryUsage),
				"disk_usage":       services.FormatMetricValue(metric.DiskUsage),
				"network_upload":   services.FormatMetricValue(metric.NetworkUpload),
				"network_download": services.FormatMetricValue(metric.NetworkDownload),
			}
		} else {
			serverData["metrics"] = map[string]interface{}{
				"cpu_usage":        0.0,
				"memory_usage":     0.0,
				"disk_usage":       0.0,
				"network_upload":   0.0,
				"network_download": 0.0,
			}
		}

		// 计算总存储容量
		totalStorageBytes := int64(0)
		for _, disk := range server.ServerDisks {
			totalStorageBytes += disk.TotalSize
		}
		serverData["total_storage"] = utils.FormatStorageSize(totalStorageBytes)

		// 概览卡片需要 swap 摘要，无需额外跳详情接口
		if server.ServerSwap != nil {
			var swapUsagePercent float64
			if server.ServerSwap.SwapTotal > 0 {
				swapUsagePercent = float64(server.ServerSwap.SwapUsed) / float64(server.ServerSwap.SwapTotal) * 100
			}
			serverData["swap"] = map[string]interface{}{
				"swap_total":         server.ServerSwap.SwapTotal,
				"swap_used":          server.ServerSwap.SwapUsed,
				"swap_free":          server.ServerSwap.SwapFree,
				"swap_usage_percent": swapUsagePercent,
				"timestamp":          server.ServerSwap.Timestamp,
			}
		} else {
			serverData["swap"] = nil
		}

		// 根据角色和设置过滤敏感信息
		if shouldHideSensitive {
			serverData["ip"] = "***"
		}

		// 如果是管理员，添加 agent_version
		if isAdmin {
			serverData["agent_version"] = server.AgentVersion
		}

		// 公开展示策略：字段级过滤（仅 guest）
		if applyPublicDisplay {
			fields := publicDisplayCfg.Fields
			if !fields.ShowLocation {
				serverData["location"] = ""
			}
			if !fields.ShowOS {
				serverData["os"] = ""
				serverData["system_name"] = ""
			}
			if !fields.ShowArchitecture {
				serverData["architecture"] = ""
			}
			// 公开页不暴露核心数与磁盘容量
			delete(serverData, "cores")
			delete(serverData, "total_storage")
			if !fields.ShowCores {
				serverData["cpu_name"] = ""
			}
			if !fields.ShowNetworkIO {
				if m, ok := serverData["metrics"].(map[string]interface{}); ok {
					m["network_upload"] = 0.0
					m["network_download"] = 0.0
				}
			}
			if !fields.ShowBilling {
				delete(serverData, "billing")
			} else if !fields.ShowTraffic {
				// 仅隐藏“流量/带宽相关”，保留计费周期/价格/到期
				if b, ok := serverData["billing"].(map[string]interface{}); ok {
					delete(b, "bandwidth_mbps")
					delete(b, "traffic_limit_type")
					delete(b, "traffic_limit_bytes")
					delete(b, "traffic_reset_cycle")
					delete(b, "traffic_custom_cycle_days")
				}
				if n, ok := serverData["network"].(map[string]interface{}); ok {
					n["show_traffic_limit"] = false
					n["show_traffic_reset_cycle"] = false
				}
			}
		}

		servers = append(servers, serverData)
	}

	return utils.SuccessResponse(ctx, "获取成功", servers)
}

// GetServerDetail 获取服务器详细信息
func (c *ServerController) GetServerDetail(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	// 获取用户类型
	userType, _ := ctx.Value("user_type").(string)
	if userType == "" {
		userType = "guest" // 默认为游客
	}
	isAdmin := userType == "admin"

	serverRepo := repositories.GetServerRepository()
	server, err := serverRepo.GetByIDWithRelations(serverID)
	if err != nil {
		facades.Log().Errorf("获取服务器详情失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "获取服务器详情失败", err)
	}

	if server == nil {
		return utils.ErrorResponse(ctx, http.StatusNotFound, "服务器不存在")
	}

	serverData := map[string]interface{}{
		"id":               server.ID,
		"name":             server.Name,
		"ip":               server.IP,
		"location":         server.Location,
		"status":           server.Status,
		"os":               server.OS,
		"architecture":     server.Architecture,
		"kernel":           server.Kernel,
		"hostname":         server.Hostname,
		"cores":            server.Cores,
		"system_name":      server.SystemName,
		"cpu_name":         server.CpuName,
		"boot_time":        server.BootTime,
		"last_report_time": server.LastReportTime,
		"uptime_days":      server.UptimeDays,
		"created_at":       server.CreatedAt.Format("2006-01-02 15:04:05"),
		"updated_at":       server.UpdatedAt.Format("2006-01-02 15:04:05"),
		"service_status":   server.ServiceStatus,
		"gpu_info":         server.GPUInfo,
	}

	revealAgentKey := strings.EqualFold(ctx.Request().Query("reveal_agent_key", ""), "true") ||
		ctx.Request().Query("reveal_agent_key", "") == "1"
	if isAdmin && revealAgentKey {
		serverData["agent_key"] = server.AgentKey
		// agent_key 是可控制服务器的凭据，明文回显必须留审计痕迹
		facades.Log().Infof("Agent 密钥已明文回显: server_id=%s ip=%s", serverID, ctx.Request().Ip())
	} else {
		serverData["agent_key_masked"] = maskAgentKey(server.AgentKey)
	}

	// 添加分组和付费信息
	if server.GroupID != nil {
		serverData["group_id"] = *server.GroupID
		if server.ServerGroup != nil {
			serverData["group"] = map[string]interface{}{
				"id":          server.ServerGroup.ID,
				"name":        server.ServerGroup.Name,
				"description": server.ServerGroup.Description,
				"color":       server.ServerGroup.Color,
			}
		}
	}

	billingData := map[string]interface{}{
		"show_billing_cycle": server.ShowBillingCycle,
	}

	networkData := map[string]interface{}{
		"show_traffic_limit":       server.ShowTrafficLimit,
		"show_traffic_reset_cycle": server.ShowTrafficResetCycle,
	}
	serverData["network"] = networkData

	if server.BillingCycle != "" {
		billingData["billing_cycle"] = server.BillingCycle
	}
	if server.CustomCycleDays != nil {
		billingData["custom_cycle_days"] = *server.CustomCycleDays
	}
	if server.Price != nil {
		billingData["price"] = *server.Price
	}
	if server.ExpireTime != nil {
		billingData["expire_time"] = server.ExpireTime.Format("2006-01-02 15:04:05")
	}
	if server.BandwidthMbps > 0 {
		billingData["bandwidth_mbps"] = server.BandwidthMbps
	}
	if server.TrafficLimitType != "" {
		billingData["traffic_limit_type"] = server.TrafficLimitType
	}
	if server.TrafficLimitBytes > 0 {
		billingData["traffic_limit_bytes"] = server.TrafficLimitBytes
	}
	if resolvedTrafficCycle := resolveTrafficCycleForResponse(
		server.TrafficLimitType,
		server.TrafficResetCycle,
	); resolvedTrafficCycle != "" {
		billingData["traffic_reset_cycle"] = resolvedTrafficCycle
	}
	if server.TrafficCustomCycleDays != nil {
		billingData["traffic_custom_cycle_days"] = *server.TrafficCustomCycleDays
	}
	serverData["billing"] = billingData

	// 计算运行时间
	serverData["uptime"] = services.CalculateUptime(server.BootTime, nil)

	// 处理磁盘信息
	disks := make([]map[string]interface{}, 0, len(server.ServerDisks))
	for _, disk := range server.ServerDisks {
		diskData := map[string]interface{}{
			"disk_name":   disk.DiskName,
			"mount_point": disk.MountPoint,
			"total_size":  disk.TotalSize,
			"used_size":   disk.UsedSize,
			"free_size":   disk.FreeSize,
		}
		if disk.TotalSize > 0 {
			diskData["usage_percent"] = float64(disk.UsedSize) / float64(disk.TotalSize) * 100
		}
		disks = append(disks, diskData)
	}
	serverData["disks"] = disks
	serverData["cpus"] = []map[string]interface{}{}

	// 处理内存信息
	if len(server.ServerMemoryHistory) > 0 {
		mem := server.ServerMemoryHistory[0]
		serverData["memory"] = map[string]interface{}{
			"memory_total":         mem.MemoryTotal,
			"memory_used":          mem.MemoryUsed,
			"memory_usage_percent": mem.MemoryUsagePercent,
			"timestamp":            mem.Timestamp,
		}
	} else {
		serverData["memory"] = nil
	}

	// 处理Swap信息
	if server.ServerSwap != nil {
		var usagePercent float64
		if server.ServerSwap.SwapTotal > 0 {
			usagePercent = float64(server.ServerSwap.SwapUsed) / float64(server.ServerSwap.SwapTotal) * 100
		}
		serverData["swap"] = map[string]interface{}{
			"swap_total":         server.ServerSwap.SwapTotal,
			"swap_used":          server.ServerSwap.SwapUsed,
			"swap_free":          server.ServerSwap.SwapFree,
			"swap_usage_percent": usagePercent,
			"timestamp":          server.ServerSwap.Timestamp,
		}
	} else {
		serverData["swap"] = nil
	}

	// 查询自开机以来的总流量统计
	var totalTraffic []map[string]interface{}
	err = facades.Orm().Query().Raw(
		"SELECT SUM(upload_bytes) as upload_bytes, SUM(download_bytes) as download_bytes FROM server_traffic_usage WHERE server_id = ?",
		serverID,
	).Scan(&totalTraffic)

	if err == nil && len(totalTraffic) > 0 {
		uploadBytes := totalTraffic[0]["upload_bytes"]
		downloadBytes := totalTraffic[0]["download_bytes"]
		if uploadBytes == nil {
			uploadBytes = 0
		}
		if downloadBytes == nil {
			downloadBytes = 0
		}
		serverData["traffic"] = map[string]interface{}{
			"upload_bytes":   uploadBytes,
			"download_bytes": downloadBytes,
		}
	} else {
		serverData["traffic"] = map[string]interface{}{
			"upload_bytes":   0,
			"download_bytes": 0,
		}
	}

	// 如果是管理员，添加 agent_version
	if isAdmin {
		serverData["agent_version"] = server.AgentVersion
	}

	// 获取告警规则
	alertService := services.NewAlertService()
	serverIDPtr := &serverID
	rules, err := alertService.GetServerRules(serverIDPtr)
	if err == nil {
		alertRulesData := map[string]interface{}{
			"cpu": map[string]interface{}{
				"enabled":  rules.CPU.Enabled,
				"warning":  rules.CPU.Warning,
				"critical": rules.CPU.Critical,
			},
			"memory": map[string]interface{}{
				"enabled":  rules.Memory.Enabled,
				"warning":  rules.Memory.Warning,
				"critical": rules.Memory.Critical,
			},
			"disk": map[string]interface{}{
				"enabled":  rules.Disk.Enabled,
				"warning":  rules.Disk.Warning,
				"critical": rules.Disk.Critical,
			},
		}

		// 获取其他类型的告警规则（bandwidth, traffic, expiration）
		ruleRepo := repositories.GetServerAlertRuleRepository()

		// bandwidth: {enabled: bool, threshold: number}
		bandwidthRule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "bandwidth")
		if err == nil && bandwidthRule != nil {
			var ruleConfig map[string]interface{}
			if err := json.Unmarshal([]byte(bandwidthRule.Config), &ruleConfig); err == nil {
				alertRulesData["bandwidth"] = ruleConfig
			} else {
				// 解析失败，返回默认值
				alertRulesData["bandwidth"] = map[string]interface{}{
					"enabled":   false,
					"threshold": 100,
				}
			}
		} else {
			// 没有配置，返回默认禁用状态
			alertRulesData["bandwidth"] = map[string]interface{}{
				"enabled":   false,
				"threshold": 100,
			}
		}

		// traffic: {enabled: bool, threshold_percent: number}
		trafficRule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "traffic")
		if err == nil && trafficRule != nil {
			var ruleConfig map[string]interface{}
			if err := json.Unmarshal([]byte(trafficRule.Config), &ruleConfig); err == nil {
				alertRulesData["traffic"] = ruleConfig
			} else {
				// 解析失败，返回默认值
				alertRulesData["traffic"] = map[string]interface{}{
					"enabled":           false,
					"threshold_percent": 80,
				}
			}
		} else {
			// 没有配置，返回默认禁用状态
			alertRulesData["traffic"] = map[string]interface{}{
				"enabled":           false,
				"threshold_percent": 80,
			}
		}

		// expiration: {enabled: bool, alert_days: number}
		expirationRule, err := ruleRepo.GetByServerIDAndType(serverIDPtr, "expiration")
		if err == nil && expirationRule != nil {
			var ruleConfig map[string]interface{}
			if err := json.Unmarshal([]byte(expirationRule.Config), &ruleConfig); err == nil {
				alertRulesData["expiration"] = ruleConfig
			} else {
				// 解析失败，返回默认值
				alertRulesData["expiration"] = map[string]interface{}{
					"enabled":    false,
					"alert_days": 7,
				}
			}
		} else {
			// 没有配置，返回默认禁用状态
			alertRulesData["expiration"] = map[string]interface{}{
				"enabled":    false,
				"alert_days": 7,
			}
		}

		serverData["alert_rules"] = alertRulesData
	}

	// 获取服务器通知渠道配置
	notificationChannels, err := alertService.GetServerNotificationChannels(serverID)
	if err == nil {
		serverData["notification_channels"] = notificationChannels
	}

	// 添加Agent配置字段
	if server.AgentTimezone != "" {
		serverData["agent_timezone"] = server.AgentTimezone
	}
	serverData["agent_metrics_interval"] = server.AgentMetricsInterval
	serverData["agent_detail_interval"] = server.AgentDetailInterval
	serverData["agent_system_interval"] = server.AgentSystemInterval
	serverData["agent_heartbeat_interval"] = server.AgentHeartbeatInterval
	if server.AgentLogPath != "" {
		serverData["agent_log_path"] = server.AgentLogPath
	}

	// 添加显示开关字段
	// serverData["show_billing_cycle"] = server.ShowBillingCycle
	// serverData["show_traffic_limit"] = server.ShowTrafficLimit
	// serverData["show_traffic_reset_cycle"] = server.ShowTrafficResetCycle

	return utils.SuccessResponse(ctx, "获取成功", serverData)
}

// UpdateServer 更新服务器信息
func (c *ServerController) UpdateServer(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	type RuleInput struct {
		Enabled  bool    `json:"enabled" form:"enabled"`
		Warning  float64 `json:"warning" form:"warning"`
		Critical float64 `json:"critical" form:"critical"`
	}

	type BillingUpdateRequest struct {
		BillingCycle           string   `json:"billing_cycle" form:"billing_cycle"`
		CustomCycleDays        *int     `json:"custom_cycle_days" form:"custom_cycle_days"`
		Price                  *float64 `json:"price" form:"price"`
		ExpireTime             *string  `json:"expire_time" form:"expire_time"`
		BandwidthMbps          int      `json:"bandwidth_mbps" form:"bandwidth_mbps"`
		TrafficLimitType       string   `json:"traffic_limit_type" form:"traffic_limit_type"`
		TrafficLimitBytes      int64    `json:"traffic_limit_bytes" form:"traffic_limit_bytes"`
		TrafficResetCycle      string   `json:"traffic_reset_cycle" form:"traffic_reset_cycle"`
		TrafficCustomCycleDays *int     `json:"traffic_custom_cycle_days" form:"traffic_custom_cycle_days"`
		// 显示开关字段
		ShowBillingCycle *bool `json:"show_billing_cycle" form:"show_billing_cycle"`
	}

	type NetworkUpdateRequest struct {
		ShowTrafficLimit      *bool `json:"show_traffic_limit" form:"show_traffic_limit"`
		ShowTrafficResetCycle *bool `json:"show_traffic_reset_cycle" form:"show_traffic_reset_cycle"`
	}

	type UpdateServerRequest struct {
		Name                 string                  `json:"name" form:"name"`
		IP                   string                  `json:"ip" form:"ip"`
		Location             string                  `json:"location" form:"location"`
		OS                   string                  `json:"os" form:"os"`
		GroupID              *uint                   `json:"group_id" form:"group_id"`
		ClearGroup           bool                    `json:"clear_group" form:"clear_group"` // 为 true 时清除服务器分组
		Billing              *BillingUpdateRequest   `json:"billing" form:"billing"`
		Network              *NetworkUpdateRequest   `json:"network" form:"network"`
		AlertRules           *map[string]interface{} `json:"alert_rules" form:"alert_rules"`
		NotificationChannels *map[string]bool        `json:"notification_channels" form:"notification_channels"`
		// Agent配置字段
		AgentTimezone          *string   `json:"agent_timezone" form:"agent_timezone"`
		AgentMetricsInterval   *int      `json:"agent_metrics_interval" form:"agent_metrics_interval"`
		AgentDetailInterval    *int      `json:"agent_detail_interval" form:"agent_detail_interval"`
		AgentSystemInterval    *int      `json:"agent_system_interval" form:"agent_system_interval"`
		AgentHeartbeatInterval *int      `json:"agent_heartbeat_interval" form:"agent_heartbeat_interval"`
		AgentLogPath           *string   `json:"agent_log_path" form:"agent_log_path"`
		MonitoredServices      *[]string `json:"monitored_services" form:"monitored_services"`
	}

	var req UpdateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
	}

	// 构建更新数据
	updateData := make(map[string]interface{})
	if req.Name != "" {
		updateData["name"] = req.Name
	}
	if req.IP != "" {
		updateData["ip"] = req.IP
	}
	if req.Location != "" {
		updateData["location"] = req.Location
	}
	if req.OS != "" {
		updateData["os"] = req.OS
	}
	// 分组相关字段
	if req.ClearGroup {
		updateData["group_id"] = nil
	} else if req.GroupID != nil {
		updateData["group_id"] = *req.GroupID
	}

	// 付费相关字段
	if req.Billing != nil {
		if req.Billing.BillingCycle != "" {
			updateData["billing_cycle"] = req.Billing.BillingCycle
		}
		if req.Billing.CustomCycleDays != nil {
			updateData["custom_cycle_days"] = *req.Billing.CustomCycleDays
		}
		if req.Billing.Price != nil {
			updateData["price"] = *req.Billing.Price
		}
		if req.Billing.ExpireTime != nil && *req.Billing.ExpireTime != "" {
			if parsed := parseExpireTime(*req.Billing.ExpireTime); parsed != nil {
				updateData["expire_time"] = *parsed
			}
		}
		if req.Billing.BandwidthMbps > 0 {
			updateData["bandwidth_mbps"] = req.Billing.BandwidthMbps
		}
		if req.Billing.TrafficLimitBytes > 0 {
			updateData["traffic_limit_bytes"] = req.Billing.TrafficLimitBytes
		}
		if req.Billing.TrafficLimitType != "" || req.Billing.TrafficResetCycle != "" {
			normalizedTrafficLimitType, normalizedTrafficResetCycle, normalizedTrafficCustomCycleDays :=
				normalizeTrafficSettings(
					req.Billing.TrafficLimitType,
					req.Billing.TrafficResetCycle,
					req.Billing.TrafficCustomCycleDays,
				)
			updateData["traffic_limit_type"] = normalizedTrafficLimitType
			updateData["traffic_reset_cycle"] = normalizedTrafficResetCycle
			if normalizedTrafficCustomCycleDays != nil {
				updateData["traffic_custom_cycle_days"] = *normalizedTrafficCustomCycleDays
			} else {
				updateData["traffic_custom_cycle_days"] = nil
			}
		} else if req.Billing.TrafficCustomCycleDays != nil {
			updateData["traffic_custom_cycle_days"] = *req.Billing.TrafficCustomCycleDays
		}
		// 处理显示开关字段
		if req.Billing.ShowBillingCycle != nil {
			updateData["show_billing_cycle"] = *req.Billing.ShowBillingCycle
		}
	}

	if req.Network != nil {
		if req.Network.ShowTrafficLimit != nil {
			updateData["show_traffic_limit"] = *req.Network.ShowTrafficLimit
		}
		if req.Network.ShowTrafficResetCycle != nil {
			updateData["show_traffic_reset_cycle"] = *req.Network.ShowTrafficResetCycle
		}
	}

	// 处理Agent配置字段
	// 用于发送给Agent的配置更新
	configUpdate := make(map[string]interface{})

	if req.AgentTimezone != nil {
		updateData["agent_timezone"] = *req.AgentTimezone
		configUpdate["agent_timezone"] = *req.AgentTimezone
	}
	if req.AgentMetricsInterval != nil {
		if *req.AgentMetricsInterval > 0 {
			updateData["agent_metrics_interval"] = *req.AgentMetricsInterval
			configUpdate["agent_metrics_interval"] = *req.AgentMetricsInterval
		}
	}
	if req.AgentDetailInterval != nil {
		if *req.AgentDetailInterval > 0 {
			updateData["agent_detail_interval"] = *req.AgentDetailInterval
			configUpdate["agent_detail_interval"] = *req.AgentDetailInterval
		}
	}
	if req.AgentSystemInterval != nil {
		if *req.AgentSystemInterval > 0 {
			updateData["agent_system_interval"] = *req.AgentSystemInterval
			configUpdate["agent_system_interval"] = *req.AgentSystemInterval
		}
	}
	if req.AgentHeartbeatInterval != nil {
		if *req.AgentHeartbeatInterval > 0 {
			updateData["agent_heartbeat_interval"] = *req.AgentHeartbeatInterval
			configUpdate["agent_heartbeat_interval"] = *req.AgentHeartbeatInterval
		}
	}
	if req.AgentLogPath != nil {
		updateData["agent_log_path"] = *req.AgentLogPath
		configUpdate["agent_log_path"] = *req.AgentLogPath
	}
	if req.MonitoredServices != nil {
		updateData["monitored_services"] = *req.MonitoredServices
		configUpdate["monitored_services"] = *req.MonitoredServices
	}

	updateData["updated_at"] = time.Now()

	// 更新数据库
	if err := repositories.GetServerRepository().Update(serverID, updateData); err != nil {
		facades.Log().Errorf("更新服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "更新服务器失败", err)
	}

	// 处理告警规则和通知渠道
	alertService := services.NewAlertService()
	if req.AlertRules != nil {
		serverIDPtr := &serverID
		rules := make(map[string]services.Rule)

		// 处理基础资源规则（cpu, memory, disk）
		// 无论 enabled 是 true 还是 false，只要规则数据存在就保存
		if cpuRule, ok := (*req.AlertRules)["cpu"].(map[string]interface{}); ok {
			enabled := false
			if e, ok := cpuRule["enabled"].(bool); ok {
				enabled = e
			}
			warning, _ := cpuRule["warning"].(float64)
			critical, _ := cpuRule["critical"].(float64)
			// 如果没有设置阈值，使用默认值
			if warning == 0 {
				warning = 80
			}
			if critical == 0 {
				critical = 90
			}
			rules["cpu"] = services.Rule{
				Enabled:  enabled,
				Warning:  warning,
				Critical: critical,
			}
		}
		if memoryRule, ok := (*req.AlertRules)["memory"].(map[string]interface{}); ok {
			enabled := false
			if e, ok := memoryRule["enabled"].(bool); ok {
				enabled = e
			}
			warning, _ := memoryRule["warning"].(float64)
			critical, _ := memoryRule["critical"].(float64)
			// 如果没有设置阈值，使用默认值
			if warning == 0 {
				warning = 85
			}
			if critical == 0 {
				critical = 95
			}
			rules["memory"] = services.Rule{
				Enabled:  enabled,
				Warning:  warning,
				Critical: critical,
			}
		}
		if diskRule, ok := (*req.AlertRules)["disk"].(map[string]interface{}); ok {
			enabled := false
			if e, ok := diskRule["enabled"].(bool); ok {
				enabled = e
			}
			warning, _ := diskRule["warning"].(float64)
			critical, _ := diskRule["critical"].(float64)
			// 如果没有设置阈值，使用默认值
			if warning == 0 {
				warning = 85
			}
			if critical == 0 {
				critical = 95
			}
			rules["disk"] = services.Rule{
				Enabled:  enabled,
				Warning:  warning,
				Critical: critical,
			}
		}

		// 保存基础资源规则
		if len(rules) > 0 {
			if err := alertService.SaveServerRules(serverIDPtr, rules); err != nil {
				facades.Log().Warningf("保存告警规则失败: %v", err)
			}
		}

		// 处理其他类型的告警规则（bandwidth, traffic, expiration）
		ruleRepo := repositories.GetServerAlertRuleRepository()
		ruleTypes := []string{"bandwidth", "traffic", "expiration"}
		for _, ruleType := range ruleTypes {
			if ruleData, ok := (*req.AlertRules)[ruleType].(map[string]interface{}); ok {
				configJson, err := json.Marshal(ruleData)
				if err == nil {
					rule := &models.ServerAlertRule{
						ServerID: serverIDPtr,
						RuleType: ruleType,
						Config:   string(configJson),
					}
					if err := ruleRepo.CreateOrUpdate(rule); err != nil {
						facades.Log().Warningf("保存告警规则 %s 失败: %v", ruleType, err)
					}
				}
			}
		}
	}

	// 处理服务器通知渠道配置
	if req.NotificationChannels != nil {
		channels := make(map[string]bool)
		for k, v := range *req.NotificationChannels {
			channels[k] = v
		}
		if err := alertService.SaveServerNotificationChannels(serverID, channels); err != nil {
			facades.Log().Warningf("保存服务器通知渠道配置失败: %v", err)
		}
	}

	// 如果更新了Agent配置，通过WebSocket发送配置更新消息
	agentConfigUpdated := req.AgentTimezone != nil || req.AgentMetricsInterval != nil ||
		req.AgentDetailInterval != nil || req.AgentSystemInterval != nil ||
		req.AgentHeartbeatInterval != nil || req.AgentLogPath != nil

	if agentConfigUpdated {
		configData := make(map[string]interface{})

		// 获取更新后的配置值
		serverRepo := repositories.GetServerRepository()
		updatedServer, err := serverRepo.GetByID(serverID)
		if err == nil && updatedServer != nil {
			if updatedServer.AgentTimezone != "" {
				configData["timezone"] = updatedServer.AgentTimezone
			}
			if updatedServer.AgentMetricsInterval > 0 {
				configData["metrics_interval"] = updatedServer.AgentMetricsInterval
			}
			if updatedServer.AgentDetailInterval > 0 {
				configData["detail_interval"] = updatedServer.AgentDetailInterval
			}
			if updatedServer.AgentSystemInterval > 0 {
				configData["system_interval"] = updatedServer.AgentSystemInterval
			}
			if updatedServer.AgentHeartbeatInterval > 0 {
				configData["heartbeat_interval"] = updatedServer.AgentHeartbeatInterval
			}
			if updatedServer.AgentLogPath != "" {
				configData["log_path"] = updatedServer.AgentLogPath
			}

			// 发送配置更新消息（签名 + 加密分派统一走 AgentCommandSender）；
			// update_config 会改写 Agent 本地配置（含日志路径），签名失败时跳过下发
			if err := services.SendSignedAgentCommand(serverID, "update_config", "", configData); err != nil {
				facades.Log().Warningf("发送Agent配置更新消息失败: %v", err)
			} else {
				facades.Log().Infof("成功发送Agent配置更新消息到服务器: %s", serverID)
			}
		}
	}

	facades.Log().Infof("成功更新服务器: %s", serverID)

	return utils.SuccessResponse(ctx, "更新成功")
}

// DeleteServer 删除服务器
func (c *ServerController) DeleteServer(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	// 删除前先断开 WS 连接：否则已连接 Agent 会继续上报，
	// 为已删除的 server_id 重新写入指标数据（"幽灵服务器"复活）
	services.GetWebSocketService().GetManager().UnregisterAgent(serverID)

	// 关联数据清理下沉到 repository，事务内完成（含 agent 任务/日志、
	// 告警状态、server 来源事件、service_monitors.server_ids 移除）
	if err := repositories.NewServerRepository().DeleteCascade(serverID); err != nil {
		facades.Log().Errorf("删除服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "删除服务器失败", err)
	}

	facades.Log().Infof("成功删除服务器: %s", serverID)

	return utils.SuccessResponse(ctx, "删除成功")
}

// RestartAgent 重启服务器agent
func (c *ServerController) RestartAgent(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	// 通过WebSocket向agent发送重启命令（签名 + 加密分派统一走 AgentCommandSender；
	// 签名失败时拒绝下发）
	if err := services.SendSignedAgentCommand(serverID, "restart", "", map[string]interface{}{}); err != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "重启Agent命令失败: "+err.Error())
	}

	facades.Log().Infof("成功发送重启命令到服务器: %s", serverID)

	return utils.SuccessResponse(ctx, "重启命令已发送")
}

// ResetAgentKey 重置服务器通信密钥
func (c *ServerController) ResetAgentKey(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID")
	}

	// 验证服务器是否存在
	serverRepo := repositories.NewServerRepository()
	_, err := serverRepo.GetByID(serverID)
	if err != nil {
		facades.Log().Errorf("获取服务器信息失败: %v", err)
		return utils.ErrorResponse(ctx, http.StatusNotFound, "服务器不存在")
	}

	// 生成新的 agent_key
	newAgentKey := uuid.New().String()

	// 重置 agent_key 并清除指纹和公钥
	err = serverRepo.Update(serverID, map[string]interface{}{
		"agent_key":         newAgentKey,
		"agent_key_hash":    services.HashAgentKey(newAgentKey),
		"agent_public_key":  nil,
		"agent_fingerprint": nil,
	})
	if err != nil {
		facades.Log().Errorf("更新通信密钥失败: %v", err)
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "重置通信密钥失败: "+err.Error())
	}

	// 断开该服务器的 WebSocket 连接
	services.GetWebSocketService().GetManager().UnregisterAgent(serverID)

	facades.Log().Infof("成功重置服务器通信密钥: %s", serverID)

	return utils.SuccessResponse(ctx, "通信密钥和指纹已重置", map[string]interface{}{
		"agent_key": newAgentKey,
	})
}

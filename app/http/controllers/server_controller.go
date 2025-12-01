package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

// calculateUptime 计算运行时间
func calculateUptime(bootTimeVal interface{}) string {
	return services.CalculateUptime(bootTimeVal)
}

type ServerController struct{}

func NewServerController() *ServerController {
	return &ServerController{}
}

// CreateServer 创建服务器
func (c *ServerController) CreateServer(ctx http.Context) http.Response {
	type CreateServerRequest struct {
		Name                   string   `json:"name" form:"name"`
		IP                     string   `json:"ip" form:"ip"`
		Port                   int      `json:"port" form:"port"`
		Location               string   `json:"location" form:"location"`
		OS                     string   `json:"os" form:"os"`
		GroupID                *uint    `json:"group_id" form:"group_id"`
		BillingCycle           string   `json:"billing_cycle" form:"billing_cycle"`
		CustomCycleDays        *int     `json:"custom_cycle_days" form:"custom_cycle_days"`
		Price                  *float64 `json:"price" form:"price"`
		ExpireTime             *string  `json:"expire_time" form:"expire_time"`
		BandwidthMbps          int      `json:"bandwidth_mbps" form:"bandwidth_mbps"`
		TrafficLimitType       string   `json:"traffic_limit_type" form:"traffic_limit_type"`
		TrafficLimitBytes      int64    `json:"traffic_limit_bytes" form:"traffic_limit_bytes"`
		TrafficResetCycle      string   `json:"traffic_reset_cycle" form:"traffic_reset_cycle"`
		TrafficCustomCycleDays *int     `json:"traffic_custom_cycle_days" form:"traffic_custom_cycle_days"`
	}

	var req CreateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusBadRequest, "请求参数错误", err)
	}

	// 验证必填字段
	if req.Name == "" || req.IP == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "名称和IP地址为必填项")
	}

	// 设置默认端口
	if req.Port == 0 {
		req.Port = 22
	}

	// 验证端口范围
	if req.Port < 1 || req.Port > 65535 {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "端口号必须在1-65535之间")
	}

	// 生成UUID作为server_id
	serverID := uuid.New().String()

	// 生成agent_key
	agentKey := uuid.New().String()

	// 解析到期时间
	var expireTime *time.Time
	if req.ExpireTime != nil && *req.ExpireTime != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", *req.ExpireTime)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *req.ExpireTime)
		}
		if err == nil {
			expireTime = &parsed
		}
	}

	// 创建服务器模型
	server := &models.Server{
		ID:                     serverID,
		Name:                   req.Name,
		IP:                     req.IP,
		Status:                 "offline",
		AgentKey:               agentKey,
		Cores:                  1,
		GroupID:                req.GroupID,
		BillingCycle:           req.BillingCycle,
		CustomCycleDays:        req.CustomCycleDays,
		Price:                  req.Price,
		ExpireTime:             expireTime,
		BandwidthMbps:          req.BandwidthMbps,
		TrafficLimitType:       req.TrafficLimitType,
		TrafficLimitBytes:      req.TrafficLimitBytes,
		TrafficResetCycle:      req.TrafficResetCycle,
		TrafficCustomCycleDays: req.TrafficCustomCycleDays,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}

	serverRepo := repositories.GetServerRepository()
	if err := serverRepo.Create(server); err != nil {
		facades.Log().Errorf("创建服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "创建服务器失败", err)
	}

	facades.Log().Infof("成功创建服务器: %s (IP: %s)", req.Name, req.IP)

	// 返回服务器信息和agent_key
	return utils.SuccessResponseWithStatus(ctx, http.StatusCreated, "服务器创建成功", map[string]interface{}{
		"id":         server.ID,
		"name":       server.Name,
		"ip":         server.IP,
		"status":     server.Status,
		"agent_key":  server.AgentKey,
		"created_at": server.CreatedAt,
		"updated_at": server.UpdatedAt,
	})
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
			"os":           server.OS,
			"architecture": server.Architecture,
			"status":       server.Status,
			"cores":        server.Cores,
			"created_at":   server.CreatedAt,
			"updated_at":   server.UpdatedAt,
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
		if server.BillingCycle != "" {
			serverData["billing_cycle"] = server.BillingCycle
		}
		if server.CustomCycleDays != nil {
			serverData["custom_cycle_days"] = *server.CustomCycleDays
		}
		if server.Price != nil {
			serverData["price"] = *server.Price
		}
		if server.ExpireTime != nil {
			serverData["expire_time"] = server.ExpireTime.Format("2006-01-02 15:04:05")
		}
		if server.BandwidthMbps > 0 {
			serverData["bandwidth_mbps"] = server.BandwidthMbps
		}
		if server.TrafficLimitType != "" {
			serverData["traffic_limit_type"] = server.TrafficLimitType
		}
		if server.TrafficLimitBytes > 0 {
			serverData["traffic_limit_bytes"] = server.TrafficLimitBytes
		}
		if server.TrafficResetCycle != "" {
			serverData["traffic_reset_cycle"] = server.TrafficResetCycle
		}
		if server.TrafficCustomCycleDays != nil {
			serverData["traffic_custom_cycle_days"] = *server.TrafficCustomCycleDays
		}

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

		// 根据角色和设置过滤敏感信息
		if shouldHideSensitive {
			serverData["ip"] = "***"
		}

		// 如果是管理员，添加 agent_version
		if isAdmin {
			serverData["agent_version"] = server.AgentVersion
		}

		servers = append(servers, serverData)
	}

	return utils.SuccessResponse(ctx, "获取成功", servers)
}

// GetServerDetail 获取服务器详细信息
func (c *ServerController) GetServerDetail(ctx http.Context) http.Response {
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
		"status":           server.Status,
		"os":               server.OS,
		"architecture":     server.Architecture,
		"kernel":           server.Kernel,
		"hostname":         server.Hostname,
		"cores":            server.Cores,
		"system_name":      server.SystemName,
		"boot_time":        server.BootTime,
		"last_report_time": server.LastReportTime,
		"uptime_days":      server.UptimeDays,
		"agent_key":        server.AgentKey,
		"created_at":       server.CreatedAt,
		"updated_at":       server.UpdatedAt,
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
	if server.BillingCycle != "" {
		serverData["billing_cycle"] = server.BillingCycle
	}
	if server.CustomCycleDays != nil {
		serverData["custom_cycle_days"] = *server.CustomCycleDays
	}
	if server.Price != nil {
		serverData["price"] = *server.Price
	}
	if server.ExpireTime != nil {
		serverData["expire_time"] = server.ExpireTime.Format("2006-01-02 15:04:05")
	}
	if server.BandwidthMbps > 0 {
		serverData["bandwidth_mbps"] = server.BandwidthMbps
	}
	if server.TrafficLimitType != "" {
		serverData["traffic_limit_type"] = server.TrafficLimitType
	}
	if server.TrafficLimitBytes > 0 {
		serverData["traffic_limit_bytes"] = server.TrafficLimitBytes
	}
	if server.TrafficResetCycle != "" {
		serverData["traffic_reset_cycle"] = server.TrafficResetCycle
	}
	if server.TrafficCustomCycleDays != nil {
		serverData["traffic_custom_cycle_days"] = *server.TrafficCustomCycleDays
	}

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

	return utils.SuccessResponse(ctx, "获取成功", serverData)
}

// GetServerMetricsCPU 获取服务器CPU负载历史数据
func (c *ServerController) GetServerMetricsCPU(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "cpu")
}

// GetServerMetricsMemory 获取服务器内存负载历史数据
func (c *ServerController) GetServerMetricsMemory(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "memory")
}

// GetServerMetricsDisk 获取服务器磁盘读写负载历史数据
func (c *ServerController) GetServerMetricsDisk(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "disk")
}

// GetServerMetricsNetwork 获取服务器网络IO负载历史数据
func (c *ServerController) GetServerMetricsNetwork(ctx http.Context) http.Response {
	return c.getServerMetricsByType(ctx, "network")
}

// getServerMetricsByType 根据类型获取服务器历史性能指标
func (c *ServerController) getServerMetricsByType(ctx http.Context, metricType string) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 获取时间范围参数
	// 支持两种方式：start/end 日期时间参数，或 hours 参数
	var startTime time.Time
	var endTime time.Time = time.Now()

	// 优先使用 start/end 参数
	startParam := ctx.Request().Query("start", "")
	endParam := ctx.Request().Query("end", "")

	if startParam != "" {
		// 解析开始时间
		if timestamp, err := strconv.ParseInt(startParam, 10, 64); err == nil {
			startTime = time.Unix(timestamp, 0)
		} else {
			// 解析失败，尝试ISO格式
			parsedStart, err := time.Parse(time.RFC3339, startParam)
			if err != nil {
				parsedStart, err = time.Parse("2006-01-02T15:04:05Z07:00", startParam)
				if err != nil {
					parsedStart, err = time.Parse("2006-01-02 15:04:05", startParam)
				}
			}
			if err == nil {
				startTime = parsedStart
			} else {
				// 解析失败，使用默认值（最近24小时）
				startTime = time.Now().Add(-24 * time.Hour)
			}
		}
	} else {
		// 使用 hours 参数
		hours := 24
		if hoursParam := ctx.Request().Query("hours", ""); hoursParam != "" {
			var h int
			if _, err := fmt.Sscanf(hoursParam, "%d", &h); err == nil && h > 0 {
				hours = h
			}
		}
		// 限制最大时间范围为24小时
		if hours > 24 {
			hours = 24
		}
		startTime = time.Now().Add(-time.Duration(hours) * time.Hour)
	}

	if endParam != "" {
		// 解析结束时间
		if timestamp, err := strconv.ParseInt(endParam, 10, 64); err == nil {
			endTime = time.Unix(timestamp, 0)
		} else {
			// 解析失败，尝试ISO格式
			parsedEnd, err := time.Parse(time.RFC3339, endParam)
			if err != nil {
				parsedEnd, err = time.Parse("2006-01-02T15:04:05Z07:00", endParam)
				if err != nil {
					parsedEnd, err = time.Parse("2006-01-02 15:04:05", endParam)
				}
			}
			if err == nil {
				endTime = parsedEnd
			}
		}
	}

	var metrics []map[string]interface{}
	// 初始化metrics为空切片，避免"model value required"错误
	metrics = []map[string]interface{}{}

	var err error

	// 计算时间范围（分钟）
	durationMinutes := int(endTime.Sub(startTime).Minutes())
	if durationMinutes <= 0 {
		durationMinutes = 60 // 默认1小时
	}

	// 根据时间范围计算采样间隔（分钟）
	var sampleIntervalMinutes int
	if durationMinutes <= 60 {
		// 1小时：每2分钟一个点，约30个点
		sampleIntervalMinutes = 2
	} else if durationMinutes <= 180 {
		// 3小时：每5分钟一个点，约36个点
		sampleIntervalMinutes = 5
	} else if durationMinutes <= 360 {
		// 6小时：每10分钟一个点，约36个点
		sampleIntervalMinutes = 10
	} else if durationMinutes <= 720 {
		// 12小时：每30分钟一个点，约24个点
		sampleIntervalMinutes = 30
	} else {
		// 24小时：每30分钟一个点，约48个点
		sampleIntervalMinutes = 30
	}

	switch metricType {
	case "cpu":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(cpu_usage) AS cpu_usage
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				cpu_usage
			FROM server_metrics
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "memory":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(memory_usage_percent) AS memory_usage
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				memory_usage_percent
			FROM server_memory_history
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "disk":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(read_speed) AS disk_read,
			AVG(write_speed) AS disk_write
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				read_speed,
				write_speed
			FROM server_disk_io
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	case "network":
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		sql := `SELECT 
			datetime(CAST((timestamp_unix / ?) * ? AS INTEGER), 'unixepoch') AS timestamp,
			AVG(upload_speed) AS network_upload,
			AVG(download_speed) AS network_download
		FROM (
			SELECT 
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END AS timestamp_unix,
				upload_speed,
				download_speed
			FROM server_network_speed
			WHERE server_id = ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) >= ? 
			AND (
				CASE 
					WHEN typeof(timestamp) = 'integer' THEN timestamp
					ELSE CAST(strftime('%s', datetime(timestamp)) AS INTEGER)
				END
			) <= ?
		)
		GROUP BY timestamp_unix / ?
		ORDER BY timestamp ASC`
		args := []interface{}{sampleIntervalSeconds, sampleIntervalSeconds, serverID, startTime.Unix()}
		if endParam != "" {
			args = append(args, endTime.Unix())
		} else {
			args = append(args, time.Now().Unix())
		}
		args = append(args, sampleIntervalSeconds)

		err = facades.Orm().Query().Raw(sql, args...).Scan(&metrics)

	default:
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "无效的指标类型",
		})
	}

	// 如果查询失败，记录警告但返回包含0值的数据点（不返回错误）
	if err != nil {
		facades.Log().Warningf("获取服务器%s指标失败（可能没有数据）: server_id=%s, error=%v", metricType, serverID, err)
		metrics = []map[string]interface{}{}
	}

	// 如果无数据，生成包含0值的数据点，覆盖整个时间范围
	if len(metrics) == 0 {
		facades.Log().Infof("获取服务器%s指标无数据，生成0值数据点: server_id=%s, startTime=%v, endTime=%v, 采样间隔=%d分钟",
			metricType, serverID, startTime, endTime, sampleIntervalMinutes)

		// 根据采样间隔生成数据点
		sampleIntervalSeconds := sampleIntervalMinutes * 60
		currentTime := startTime

		for currentTime.Before(endTime) || currentTime.Equal(endTime) {
			dataPoint := map[string]interface{}{
				"timestamp": currentTime.Unix(),
			}

			// 根据指标类型设置不同的字段
			switch metricType {
			case "cpu":
				dataPoint["cpu_usage"] = 0.0
			case "memory":
				dataPoint["memory_usage"] = 0.0
			case "disk":
				dataPoint["disk_read"] = 0.0
				dataPoint["disk_write"] = 0.0
			case "network":
				dataPoint["network_upload"] = 0.0
				dataPoint["network_download"] = 0.0
			}

			metrics = append(metrics, dataPoint)
			currentTime = currentTime.Add(time.Duration(sampleIntervalSeconds) * time.Second)
		}
	} else {
		// 查询成功且有数据
		facades.Log().Infof("获取服务器%s指标成功: server_id=%s, 时间范围=%d分钟, 采样间隔=%d分钟, 数据量=%d, startTime=%v, endTime=%v",
			metricType, serverID, durationMinutes, sampleIntervalMinutes, len(metrics), startTime, endTime)
	}

	// 转换timestamp为Unix时间戳（秒），并处理数值字段保留两位小数（不四舍五入）
	for i := range metrics {
		if ts, ok := metrics[i]["timestamp"]; ok {
			var unixTimestamp int64
			switch v := ts.(type) {
			case time.Time:
				unixTimestamp = v.Unix()
			case string:
				// 尝试解析时间字符串
				if parsedTime, err := time.Parse(time.RFC3339, v); err == nil {
					unixTimestamp = parsedTime.Unix()
				} else if parsedTime, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
					unixTimestamp = parsedTime.Unix()
				} else {
					unixTimestamp = time.Now().Unix()
				}
			case int64:
				unixTimestamp = v
			case int:
				unixTimestamp = int64(v)
			case float64:
				unixTimestamp = int64(v)
			default:
				unixTimestamp = time.Now().Unix()
			}
			metrics[i]["timestamp"] = unixTimestamp
		}

		for key, value := range metrics[i] {
			if key == "timestamp" {
				continue // 跳过timestamp字段
			}
			// 使用统一的格式化函数
			metrics[i][key] = services.FormatMetricValue(value)
		}
	}

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "获取成功",
		"data":    metrics,
	})
}

// UpdateServer 更新服务器信息
func (c *ServerController) UpdateServer(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	type UpdateServerRequest struct {
		Name                   string   `json:"name" form:"name"`
		IP                     string   `json:"ip" form:"ip"`
		Port                   int      `json:"port" form:"port"`
		Location               string   `json:"location" form:"location"`
		OS                     string   `json:"os" form:"os"`
		GroupID                *uint    `json:"group_id" form:"group_id"`
		BillingCycle           string   `json:"billing_cycle" form:"billing_cycle"`
		CustomCycleDays        *int     `json:"custom_cycle_days" form:"custom_cycle_days"`
		Price                  *float64 `json:"price" form:"price"`
		ExpireTime             *string  `json:"expire_time" form:"expire_time"`
		BandwidthMbps          int      `json:"bandwidth_mbps" form:"bandwidth_mbps"`
		TrafficLimitType       string   `json:"traffic_limit_type" form:"traffic_limit_type"`
		TrafficLimitBytes      int64    `json:"traffic_limit_bytes" form:"traffic_limit_bytes"`
		TrafficResetCycle      string   `json:"traffic_reset_cycle" form:"traffic_reset_cycle"`
		TrafficCustomCycleDays *int     `json:"traffic_custom_cycle_days" form:"traffic_custom_cycle_days"`
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
	if req.Port > 0 {
		if req.Port < 1 || req.Port > 65535 {
			return utils.ErrorResponse(ctx, http.StatusBadRequest, "端口号必须在1-65535之间")
		}
		updateData["port"] = req.Port
	}
	if req.Location != "" {
		updateData["location"] = req.Location
	}
	if req.OS != "" {
		updateData["os"] = req.OS
	}
	// 付费和分组相关字段
	if req.GroupID != nil {
		updateData["group_id"] = *req.GroupID
	}
	if req.BillingCycle != "" {
		updateData["billing_cycle"] = req.BillingCycle
	}
	if req.CustomCycleDays != nil {
		updateData["custom_cycle_days"] = *req.CustomCycleDays
	}
	if req.Price != nil {
		updateData["price"] = *req.Price
	}
	if req.ExpireTime != nil && *req.ExpireTime != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", *req.ExpireTime)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *req.ExpireTime)
		}
		if err == nil {
			updateData["expire_time"] = parsed
		}
	}
	if req.BandwidthMbps > 0 {
		updateData["bandwidth_mbps"] = req.BandwidthMbps
	}
	if req.TrafficLimitType != "" {
		updateData["traffic_limit_type"] = req.TrafficLimitType
	}
	if req.TrafficLimitBytes > 0 {
		updateData["traffic_limit_bytes"] = req.TrafficLimitBytes
	}
	if req.TrafficResetCycle != "" {
		updateData["traffic_reset_cycle"] = req.TrafficResetCycle
	}
	if req.TrafficCustomCycleDays != nil {
		updateData["traffic_custom_cycle_days"] = *req.TrafficCustomCycleDays
	}
	updateData["updated_at"] = time.Now()

	// 更新数据库
	if err := repositories.GetServerRepository().Update(serverID, updateData); err != nil {
		facades.Log().Errorf("更新服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "更新服务器失败", err)
	}

	facades.Log().Infof("成功更新服务器: %s", serverID)

	return utils.SuccessResponse(ctx, "更新成功")
}

// DeleteServer 删除服务器
func (c *ServerController) DeleteServer(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 删除服务器
	_, err := facades.Orm().Query().Model(&models.Server{}).
		Where("id", serverID).
		Delete()

	if err != nil {
		facades.Log().Errorf("删除服务器失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "删除服务器失败", err)
	}

	facades.Log().Infof("成功删除服务器: %s", serverID)

	return utils.SuccessResponse(ctx, "删除成功")
}

// RestartServer 重启服务器agent
func (c *ServerController) RestartServer(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 通过WebSocket向agent发送重启命令
	wsService := services.GetWebSocketService()
	message := map[string]interface{}{
		"type":    "command",
		"command": "restart",
	}

	err := wsService.SendMessage(serverID, message)
	if err != nil {
		facades.Log().Errorf("发送重启命令失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "发送重启命令失败: " + err.Error(),
		})
	}

	facades.Log().Infof("成功发送重启命令到服务器: %s", serverID)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "重启命令已发送",
	})
}

// UpdateAgent 更新服务器 Agent
func (c *ServerController) UpdateAgent(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 获取最新版本信息
	updateType := ctx.Request().Input("type", "github")
	if updateType != "github" && updateType != "gitee" {
		updateType = "github"
	}

	// 直接获取最新版本信息
	requestUrl := "https://api.github.com/repos/YunTower/CloudSentinel-Agent/releases/latest"
	if updateType == "gitee" {
		requestUrl = "https://gitee.com/api/v5/repos/YunTower/CloudSentinel-Agent/releases/latest"
	}

	response, requestErr := facades.Http().Get(requestUrl)
	if requestErr != nil {
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "请求最新版本信息失败",
			"error":   requestErr.Error(),
		})
	}

	responseBody, responseErr := response.Body()
	if responseErr != nil {
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "读取最新版本信息失败",
			"error":   responseErr.Error(),
		})
	}

	if response.Status() == 404 {
		return ctx.Response().Status(http.StatusNotFound).Json(http.Json{
			"status":  false,
			"message": "未找到最新的版本信息",
		})
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(responseBody), &result); err != nil {
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "解析版本信息失败",
			"error":   err.Error(),
		})
	}

	tagName, ok := result["tag_name"].(string)
	if !ok {
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "版本信息格式错误",
		})
	}

	// 格式化版本号
	if len(tagName) > 0 && tagName[0] == 'v' {
		tagName = tagName[1:]
	}

	// 提取版本类型
	versionParts := strings.Split(tagName, "-")
	versionType := "release"
	if len(versionParts) > 1 {
		versionType = versionParts[1]
	}

	// 发送更新命令
	wsService := services.GetWebSocketService()
	message := map[string]interface{}{
		"type":    "command",
		"command": "update",
		"data": map[string]interface{}{
			"version":      tagName,
			"version_type": versionType,
		},
	}

	err := wsService.SendMessage(serverID, message)
	if err != nil {
		facades.Log().Errorf("发送更新命令失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "发送更新命令失败: " + err.Error(),
		})
	}

	facades.Log().Infof("成功发送更新命令到服务器: %s, 版本: %s", serverID, tagName)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "更新命令已发送",
		"data": map[string]interface{}{
			"version":      tagName,
			"version_type": versionType,
		},
	})
}

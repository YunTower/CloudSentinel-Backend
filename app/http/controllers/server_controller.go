package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"goravel/app/services"

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
		Name     string `json:"name" form:"name"`
		IP       string `json:"ip" form:"ip"`
		Port     int    `json:"port" form:"port"`
		Location string `json:"location" form:"location"`
		OS       string `json:"os" form:"os"`
	}

	var req CreateServerRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
	}

	// 验证必填字段
	if req.Name == "" || req.IP == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "名称和IP地址为必填项",
		})
	}

	// 设置默认端口
	if req.Port == 0 {
		req.Port = 22
	}

	// 验证端口范围
	if req.Port < 1 || req.Port > 65535 {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "端口号必须在1-65535之间",
		})
	}

	// 生成UUID作为server_id
	serverID := uuid.New().String()

	// 生成agent_key
	agentKey := uuid.New().String()

	now := time.Now().Unix()

	// 插入数据库
	serverData := map[string]interface{}{
		"id":         serverID,
		"name":       req.Name,
		"ip":         req.IP,
		"port":       req.Port,
		"status":     "offline",
		"location":   req.Location,
		"os":         req.OS,
		"agent_key":  agentKey,
		"cores":      1,
		"created_at": now,
		"updated_at": now,
	}

	_, err := facades.Orm().Query().Exec(
		"INSERT INTO servers (id, name, ip, port, status, location, os, agent_key, cores, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		serverData["id"], serverData["name"], serverData["ip"], serverData["port"],
		serverData["status"], serverData["location"], serverData["os"],
		serverData["agent_key"], serverData["cores"], serverData["created_at"], serverData["updated_at"],
	)

	if err != nil {
		facades.Log().Errorf("创建服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "创建服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功创建服务器: %s (IP: %s)", req.Name, req.IP)

	// 返回服务器信息和agent_key
	return ctx.Response().Status(http.StatusCreated).Json(http.Json{
		"status":  true,
		"message": "服务器创建成功",
		"data": map[string]interface{}{
			"id":         serverID,
			"name":       req.Name,
			"ip":         req.IP,
			"port":       req.Port,
			"status":     "offline",
			"location":   req.Location,
			"os":         req.OS,
			"agent_key":  agentKey,
			"created_at": now,
			"updated_at": now,
		},
	})
}

// GetServers 获取服务器列表
func (c *ServerController) GetServers(ctx http.Context) http.Response {
	// 获取用户类型
	userType, _ := ctx.Value("user_type").(string)
	if userType == "" {
		userType = "guest" // 默认为游客
	}

	// 获取敏感信息隐藏设置
	var hideSensitiveInfoStr string
	hideSensitiveInfo := true // 默认隐藏
	if err := facades.DB().Table("system_settings").
		Where("setting_key", "hide_sensitive_info").
		Value("setting_value", &hideSensitiveInfoStr); err == nil {
		hideSensitiveInfo = hideSensitiveInfoStr == "true"
	}

	// 判断是否需要隐藏敏感信息
	shouldHideSensitive := userType == "guest" && hideSensitiveInfo
	// 判断是否是管理员
	isAdmin := userType == "admin"

	var servers []map[string]interface{}
	// 根据用户类型决定是否查询 agent_version
	selectFields := []string{"id", "name", "ip", "port", "os", "architecture", "status", "location", "boot_time", "uptime_seconds", "cores", "created_at", "updated_at"}
	if isAdmin {
		selectFields = append(selectFields, "agent_version")
	}

	err := facades.Orm().Query().Table("servers").
		Select(selectFields...).
		OrderBy("created_at", "desc").
		Get(&servers)

	if err != nil {
		facades.Log().Errorf("获取服务器列表失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "获取服务器列表失败",
			"error":   err.Error(),
		})
	}

	// 为每个服务器获取最新指标和磁盘信息
	for i := range servers {
		serverID, ok := servers[i]["id"].(string)
		if !ok {
			continue
		}

		// 计算运行时间
		servers[i]["uptime"] = services.CalculateUptime(servers[i]["boot_time"], servers[i]["uptime_seconds"])

		// 获取最新指标数据
		var latestMetrics []map[string]interface{}
		facades.Orm().Query().Table("server_metrics").
			Select("cpu_usage", "memory_usage", "disk_usage", "network_upload", "network_download").
			Where("server_id", serverID).
			OrderBy("timestamp", "desc").
			Limit(1).
			Get(&latestMetrics)

		if len(latestMetrics) > 0 {
			metric := latestMetrics[0]
			servers[i]["metrics"] = map[string]interface{}{
				"cpu_usage":        services.FormatMetricValue(metric["cpu_usage"]),
				"memory_usage":     services.FormatMetricValue(metric["memory_usage"]),
				"disk_usage":       services.FormatMetricValue(metric["disk_usage"]),
				"network_upload":   services.FormatMetricValue(metric["network_upload"]),
				"network_download": services.FormatMetricValue(metric["network_download"]),
			}
		} else {
			// 如果没有指标数据，设置默认值
			servers[i]["metrics"] = map[string]interface{}{
				"cpu_usage":        0.0,
				"memory_usage":     0.0,
				"disk_usage":       0.0,
				"network_upload":   0.0,
				"network_download": 0.0,
			}
		}

		// 计算总存储容量
		var disks []map[string]interface{}
		facades.Orm().Query().Table("server_disks").
			Select("total_size").
			Where("server_id", serverID).
			Get(&disks)

		var totalStorageBytes int64 = 0
		for _, disk := range disks {
			if totalSize, ok := disk["total_size"].(int64); ok {
				totalStorageBytes += totalSize
			}
		}

		// 格式化存储容量
		var totalStorage string
		if totalStorageBytes == 0 {
			totalStorage = ""
		} else if totalStorageBytes >= 1024*1024*1024*1024 {
			// TB
			totalStorage = fmt.Sprintf("%.1fTB", float64(totalStorageBytes)/(1024*1024*1024*1024))
		} else if totalStorageBytes >= 1024*1024*1024 {
			// GB
			totalStorage = fmt.Sprintf("%.1fGB", float64(totalStorageBytes)/(1024*1024*1024))
		} else if totalStorageBytes >= 1024*1024 {
			// MB
			totalStorage = fmt.Sprintf("%.1fMB", float64(totalStorageBytes)/(1024*1024))
		} else {
			// KB
			totalStorage = fmt.Sprintf("%.1fKB", float64(totalStorageBytes)/1024)
		}
		servers[i]["total_storage"] = totalStorage

		// 根据角色和设置过滤敏感信息
		if shouldHideSensitive {
			// 隐藏IP地址和端口
			servers[i]["ip"] = "***"
			servers[i]["port"] = nil
		}

		// 如果不是管理员，移除 agent_version 字段
		if !isAdmin {
			delete(servers[i], "agent_version")
		}
	}

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "获取成功",
		"data":    servers,
	})
}

// GetServerDetail 获取服务器详细信息
func (c *ServerController) GetServerDetail(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
			"status":  false,
			"message": "缺少服务器ID",
		})
	}

	// 获取用户类型
	userType, _ := ctx.Value("user_type").(string)
	if userType == "" {
		userType = "guest" // 默认为游客
	}
	isAdmin := userType == "admin"

	var servers []map[string]interface{}
	// 根据用户类型决定是否查询 agent_version
	selectFields := []string{"id", "name", "ip", "port", "status", "location", "os", "architecture", "kernel", "hostname", "cores", "system_name", "boot_time", "uptime_seconds", "last_report_time", "uptime_days", "agent_key", "created_at", "updated_at"}
	if isAdmin {
		selectFields = append(selectFields, "agent_version")
	}

	err := facades.Orm().Query().Table("servers").
		Select(selectFields...).
		Where("id", serverID).
		Get(&servers)

	if err != nil {
		facades.Log().Errorf("获取服务器详情失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "获取服务器详情失败",
			"error":   err.Error(),
		})
	}

	if len(servers) == 0 {
		return ctx.Response().Status(http.StatusNotFound).Json(http.Json{
			"status":  false,
			"message": "服务器不存在",
		})
	}

	server := servers[0]

	// 计算运行时间
	server["uptime"] = services.CalculateUptime(server["boot_time"], server["uptime_seconds"])

	// 查询磁盘信息
	var disks []map[string]interface{}
	facades.Orm().Query().Table("server_disks").
		Select("disk_name", "mount_point", "total_size", "used_size", "free_size").
		Where("server_id", serverID).
		Get(&disks)

	// 计算每个磁盘的使用率
	for i := range disks {
		if totalSize, ok := disks[i]["total_size"].(int64); ok && totalSize > 0 {
			if usedSize, ok := disks[i]["used_size"].(int64); ok {
				usagePercent := float64(usedSize) / float64(totalSize) * 100
				disks[i]["usage_percent"] = usagePercent
			}
		}
	}
	server["disks"] = disks

	// 不再查询CPU核心信息
	server["cpus"] = []map[string]interface{}{}

	// 查询最新内存记录
	var memoryRecords []map[string]interface{}
	facades.Orm().Query().Table("server_memory_history").
		Select("memory_total", "memory_used", "memory_usage_percent", "timestamp").
		Where("server_id", serverID).
		OrderBy("timestamp", "desc").
		Limit(1).
		Get(&memoryRecords)

	if len(memoryRecords) > 0 {
		server["memory"] = memoryRecords[0]
	} else {
		server["memory"] = nil
	}

	// 查询最新Swap记录
	var swapRecords []map[string]interface{}
	facades.Orm().Query().Table("server_swap").
		Select("swap_total", "swap_used", "swap_free", "timestamp").
		Where("server_id", serverID).
		OrderBy("timestamp", "desc").
		Limit(1).
		Get(&swapRecords)

	if len(swapRecords) > 0 {
		swapRecord := swapRecords[0]
		// 计算swap使用率
		var usagePercent float64
		if total, ok := swapRecord["swap_total"].(int64); ok && total > 0 {
			if used, ok := swapRecord["swap_used"].(int64); ok {
				usagePercent = float64(used) / float64(total) * 100
			}
		}
		// 转换为前端期望的字段名
		server["swap"] = map[string]interface{}{
			"swap_total":         swapRecord["swap_total"],
			"swap_used":          swapRecord["swap_used"],
			"swap_free":          swapRecord["swap_free"],
			"swap_usage_percent": usagePercent,
			"timestamp":          swapRecord["timestamp"],
		}
	} else {
		server["swap"] = nil
	}

	// 查询自开机以来的总流量统计（所有月份的总和）
	var totalTraffic []map[string]interface{}
	err = facades.Orm().Query().Raw(
		"SELECT SUM(upload_bytes) as upload_bytes, SUM(download_bytes) as download_bytes FROM server_traffic_usage WHERE server_id = ?",
		serverID,
	).Scan(&totalTraffic)

	if err == nil && len(totalTraffic) > 0 {
		uploadBytes := totalTraffic[0]["upload_bytes"]
		downloadBytes := totalTraffic[0]["download_bytes"]
		// 如果值为nil，说明没有数据，设置为0
		if uploadBytes == nil {
			uploadBytes = 0
		}
		if downloadBytes == nil {
			downloadBytes = 0
		}
		server["traffic"] = map[string]interface{}{
			"upload_bytes":   uploadBytes,
			"download_bytes": downloadBytes,
		}
	} else {
		server["traffic"] = map[string]interface{}{
			"upload_bytes":   0,
			"download_bytes": 0,
		}
	}

	// 如果不是管理员，移除 agent_version 字段
	if !isAdmin {
		delete(server, "agent_version")
	}

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "获取成功",
		"data":    server,
	})
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
		Name     string `json:"name" form:"name"`
		IP       string `json:"ip" form:"ip"`
		Port     int    `json:"port" form:"port"`
		Location string `json:"location" form:"location"`
		OS       string `json:"os" form:"os"`
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
			return ctx.Response().Status(http.StatusBadRequest).Json(http.Json{
				"status":  false,
				"message": "端口号必须在1-65535之间",
			})
		}
		updateData["port"] = req.Port
	}
	if req.Location != "" {
		updateData["location"] = req.Location
	}
	if req.OS != "" {
		updateData["os"] = req.OS
	}
	updateData["updated_at"] = time.Now().Unix()

	// 更新数据库
	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Update(updateData)

	if err != nil {
		facades.Log().Errorf("更新服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "更新服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功更新服务器: %s", serverID)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "更新成功",
	})
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

	// 删除服务器（外键级联会自动删除相关数据）
	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Delete()

	if err != nil {
		facades.Log().Errorf("删除服务器失败: %v", err)
		return ctx.Response().Status(http.StatusInternalServerError).Json(http.Json{
			"status":  false,
			"message": "删除服务器失败",
			"error":   err.Error(),
		})
	}

	facades.Log().Infof("成功删除服务器: %s", serverID)

	return ctx.Response().Json(http.StatusOK, http.Json{
		"status":  true,
		"message": "删除成功",
	})
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

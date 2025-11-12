package services

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/facades"
)

// FormatMetricValue 格式化指标值为两位小数
func FormatMetricValue(value interface{}) float64 {
	var v float64
	switch val := value.(type) {
	case float64:
		v = val
	case float32:
		v = float64(val)
	case int:
		v = float64(val)
	case int64:
		v = float64(val)
	default:
		return 0.0
	}
	// 向下取整到两位小数：先乘以100，向下取整，再除以100
	return math.Floor(v*100) / 100.0
}

// CalculateUptime 计算运行时间
// 接受多种类型的 boot_time 值（time.Time, string, int64, float64, int），返回格式化的运行时间字符串
func CalculateUptime(bootTimeVal interface{}) string {
	if bootTimeVal == nil {
		return "0天0时0分"
	}

	var bootTime time.Time

	// 处理不同类型的 boot_time 值
	switch v := bootTimeVal.(type) {
	case time.Time:
		// 如果已经是 time.Time 类型，直接使用
		bootTime = v
	case string:
		// 如果是字符串，尝试解析
		if parsedTime, err := time.Parse(time.RFC3339, v); err == nil {
			bootTime = parsedTime
		} else if parsedTime, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			bootTime = parsedTime
		} else {
			// 尝试解析 Unix 时间戳字符串
			if unixTime, err := strconv.ParseInt(v, 10, 64); err == nil && unixTime > 0 {
				bootTime = time.Unix(unixTime, 0)
			}
		}
	case int64:
		if v > 0 {
			bootTime = time.Unix(v, 0)
		}
	case float64:
		if v > 0 {
			bootTime = time.Unix(int64(v), 0)
		}
	case int:
		if v > 0 {
			bootTime = time.Unix(int64(v), 0)
		}
	}

	// 如果成功解析了启动时间，计算运行时间
	if bootTime.IsZero() {
		return "0天0时0分"
	}

	duration := time.Since(bootTime)
	if duration <= 0 {
		return "0天0时0分"
	}

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d时%d分", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d时%d分", hours, minutes)
	} else {
		return fmt.Sprintf("%d分", minutes)
	}
}

// SaveSystemInfo 保存系统基础信息
func SaveSystemInfo(serverID string, data map[string]interface{}) error {
	now := time.Now()

	updateData := map[string]interface{}{
		"last_report_time": now.Unix(),
		"status":           "online",
		"updated_at":       now.Unix(),
	}

	// 提取系统信息字段
	if v, ok := data["agent_version"].(string); ok {
		updateData["agent_version"] = v
	}
	if v, ok := data["system_name"].(string); ok {
		updateData["system_name"] = v
	}
	if v, ok := data["os"].(string); ok {
		updateData["os"] = v
	}
	if v, ok := data["architecture"].(string); ok {
		updateData["architecture"] = v
	}
	if v, ok := data["kernel"].(string); ok {
		updateData["kernel"] = v
	}
	if v, ok := data["hostname"].(string); ok {
		updateData["hostname"] = v
	}
	if v, ok := data["cores"].(float64); ok {
		updateData["cores"] = int(v)
	}
	if v, ok := data["boot_time"].(string); ok && v != "" {
		if bootTime, err := time.Parse(time.RFC3339, v); err == nil {
			// 将 boot_time 保存为 time.Time 类型，让 ORM 自动处理 TIMESTAMP 字段
			updateData["boot_time"] = bootTime
			facades.Log().Debugf("保存 boot_time: %s (Unix: %d)", bootTime.Format(time.RFC3339), bootTime.Unix())
		} else {
			facades.Log().Warningf("解析 boot_time 失败: %v, 原始值: %s", err, v)
		}
	}

	// 更新服务器记录
	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Update(updateData)

	if err != nil {
		facades.Log().Errorf("保存系统信息失败: %v", err)
		return err
	}

	facades.Log().Infof("已保存服务器 %s 的系统信息", serverID)

	// 向前端推送系统信息更新
	go func() {
		wsService := GetWebSocketService()
		message := map[string]interface{}{
			"type": "system_info_update",
			"data": map[string]interface{}{
				"server_id": serverID,
				"data":      updateData,
			},
		}
		wsService.BroadcastToFrontend(message)
	}()

	return nil
}

// SaveMetrics 保存性能指标
func SaveMetrics(serverID string, data map[string]interface{}) error {
	now := time.Now()
	timestamp := now.Unix()

	// 更新服务器基本指标
	updateData := map[string]interface{}{
		"last_report_time": timestamp,
		"status":           "online",
		"updated_at":       timestamp,
	}

	// 计算运行天数
	if uptime, ok := data["uptime"].(string); ok {
		updateData["uptime"] = uptime
		// 可以解析uptime字符串来计算天数
	}

	_, err := facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Update(updateData)

	if err != nil {
		facades.Log().Errorf("更新服务器基本指标失败: %v", err)
		return err
	}

	// 保存到server_metrics表
	metricsData := map[string]interface{}{
		"server_id": serverID,
		"timestamp": time.Now(),
	}

	// CPU使用率
	if v, ok := data["cpu_usage"].(float64); ok {
		metricsData["cpu_usage"] = FormatMetricValue(v)
	} else {
		metricsData["cpu_usage"] = 0.0 // 默认值
	}

	// 内存使用率
	if v, ok := data["memory_usage_percent"].(float64); ok {
		metricsData["memory_usage"] = FormatMetricValue(v)
	} else {
		metricsData["memory_usage"] = 0.0 // 默认值
	}

	// 磁盘使用率
	if v, ok := data["disk_usage"].(float64); ok {
		metricsData["disk_usage"] = FormatMetricValue(v)
	} else {
		metricsData["disk_usage"] = 0.0 // 默认值
	}

	// 网络速度
	if v, ok := data["network_upload"].(float64); ok {
		metricsData["network_upload"] = FormatMetricValue(v)
	} else {
		metricsData["network_upload"] = 0.0
	}
	if v, ok := data["network_download"].(float64); ok {
		metricsData["network_download"] = FormatMetricValue(v)
	} else {
		metricsData["network_download"] = 0.0
	}

	err = facades.Orm().Query().Table("server_metrics").Create(metricsData)
	if err != nil {
		facades.Log().Errorf("保存性能指标失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的性能指标", serverID)

	// 向前端推送性能指标更新
	go func() {
		facades.Log().Infof("开始准备推送服务器 %s 的性能指标更新", serverID)
		wsService := GetWebSocketService()

		// 获取服务器boot_time以计算运行时间
		var servers []map[string]interface{}
		var uptimeStr string
		err := facades.Orm().Query().Table("servers").
			Select("boot_time").
			Where("id", serverID).
			Get(&servers)

		if err == nil && len(servers) > 0 {
			uptimeStr = CalculateUptime(servers[0]["boot_time"])
		} else {
			uptimeStr = "0天0时0分"
		}

		// 格式化数值为两位小数
		cpuUsage := FormatMetricValue(metricsData["cpu_usage"])
		memoryUsage := FormatMetricValue(metricsData["memory_usage"])
		diskUsage := FormatMetricValue(metricsData["disk_usage"])
		networkUpload := FormatMetricValue(metricsData["network_upload"])
		networkDownload := FormatMetricValue(metricsData["network_download"])

		// 推送实时指标更新
		message := map[string]interface{}{
			"type": "metrics_update",
			"data": map[string]interface{}{
				"server_id": serverID,
				"metrics": map[string]interface{}{
					"cpu_usage":        cpuUsage,
					"memory_usage":     memoryUsage,
					"disk_usage":       diskUsage,
					"network_upload":   networkUpload,
					"network_download": networkDownload,
				},
				"uptime": uptimeStr,
			},
		}
		facades.Log().Infof("准备推送 metrics_update 消息，服务器: %s", serverID)
		wsService.BroadcastToFrontend(message)
		facades.Log().Infof("已调用 BroadcastToFrontend，服务器: %s", serverID)

		// 推送实时数据点
		realtimeDataPoint := map[string]interface{}{
			"type": "metrics_realtime",
			"data": map[string]interface{}{
				"server_id": serverID,
				"timestamp": timestamp,
				"metrics": map[string]interface{}{
					"cpu_usage":        cpuUsage,
					"memory_usage":     memoryUsage,
					"disk_usage":       diskUsage,
					"network_upload":   networkUpload,
					"network_download": networkDownload,
				},
			},
		}
		wsService.BroadcastToFrontend(realtimeDataPoint)
	}()

	return nil
}

// SaveMemoryInfo 保存内存信息
func SaveMemoryInfo(serverID string, data map[string]interface{}) error {
	record := map[string]interface{}{
		"server_id": serverID,
		"timestamp": time.Now(),
	}

	if v, ok := data["memory_total"].(float64); ok {
		record["memory_total"] = int64(v)
	}
	if v, ok := data["memory_used"].(float64); ok {
		record["memory_used"] = int64(v)
	}
	if v, ok := data["memory_usage_percent"].(float64); ok {
		record["memory_usage_percent"] = v
	}

	// 使用ORM的Create方法，自动处理timestamp字段
	err := facades.Orm().Query().Table("server_memory_history").Create(record)
	if err != nil {
		facades.Log().Errorf("保存内存信息失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的内存信息", serverID)
	return nil
}

// SaveDiskInfo 保存磁盘信息
func SaveDiskInfo(serverID string, data []interface{}) error {
	facades.Orm().Query().Table("server_disks").
		Where("server_id", serverID).
		Delete()

	for _, item := range data {
		diskData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		record := map[string]interface{}{
			"server_id": serverID,
		}

		if v, ok := diskData["mount_point"].(string); ok {
			record["mount_point"] = v
		}
		if v, ok := diskData["device"].(string); ok {
			record["disk_name"] = v // 映射到disk_name列
		}
		if v, ok := diskData["total"].(float64); ok {
			record["total_size"] = int64(v) // 映射到total_size列
		}
		if v, ok := diskData["used"].(float64); ok {
			record["used_size"] = int64(v) // 映射到used_size列
		}
		if v, ok := diskData["free"].(float64); ok {
			record["free_size"] = int64(v) // 映射到free_size列
		}

		err := facades.Orm().Query().Table("server_disks").Create(record)
		if err != nil {
			facades.Log().Errorf("保存磁盘信息失败: %v", err)
			return err
		}
	}

	facades.Log().Debugf("已保存服务器 %s 的磁盘信息", serverID)
	return nil
}

// SaveNetworkInfo 保存网络信息
func SaveNetworkInfo(serverID string, data map[string]interface{}) error {
	// 保存TCP/UDP连接数
	if tcpConns, ok1 := data["tcp_connections"].(float64); ok1 {
		if udpConns, ok2 := data["udp_connections"].(float64); ok2 {
			record := map[string]interface{}{
				"server_id":       serverID,
				"tcp_connections": int(tcpConns),
				"udp_connections": int(udpConns),
				"timestamp":       time.Now(),
			}

			_, err := facades.Orm().Query().Exec("INSERT INTO server_network_connections (server_id, tcp_connections, udp_connections, timestamp) VALUES (?, ?, ?, ?)",
				record["server_id"], record["tcp_connections"], record["udp_connections"], record["timestamp"])
			if err != nil {
				facades.Log().Errorf("保存网络连接数失败: %v", err)
				return err
			}
		}
	}

	// 保存网络速度
	if upload, ok1 := data["upload_speed"].(float64); ok1 {
		if download, ok2 := data["download_speed"].(float64); ok2 {
			record := map[string]interface{}{
				"server_id":      serverID,
				"upload_speed":   upload,
				"download_speed": download,
				"timestamp":      time.Now(),
			}

			// 使用ORM的Create方法，自动处理timestamp字段
			err := facades.Orm().Query().Table("server_network_speed").Create(record)
			if err != nil {
				facades.Log().Errorf("保存网络速度失败: %v", err)
				return err
			}
		}
	}

	// 更新流量使用情况
	if uploadBytes, ok1 := data["upload_bytes"].(float64); ok1 {
		if downloadBytes, ok2 := data["download_bytes"].(float64); ok2 {
			now := time.Now()
			year := now.Year()
			month := int(now.Month())

			_, err := facades.Orm().Query().Exec(
				"INSERT OR REPLACE INTO server_traffic_usage (server_id, year, month, upload_bytes, download_bytes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, COALESCE((SELECT created_at FROM server_traffic_usage WHERE server_id = ? AND year = ? AND month = ?), ?), ?)",
				serverID, year, month, int64(uploadBytes), int64(downloadBytes),
				serverID, year, month, now, now)

			if err != nil {
				var existing []map[string]interface{}
				queryErr := facades.Orm().Query().Table("server_traffic_usage").
					Where("server_id", serverID).
					Where("year", year).
					Where("month", month).
					Get(&existing)

				if queryErr == nil && len(existing) > 0 {
					// 更新现有记录
					_, err = facades.Orm().Query().Table("server_traffic_usage").
						Where("server_id", serverID).
						Where("year", year).
						Where("month", month).
						Update(map[string]interface{}{
							"upload_bytes":   int64(uploadBytes),
							"download_bytes": int64(downloadBytes),
							"updated_at":     now,
						})
				} else {
					// 插入新记录
					err = facades.Orm().Query().Table("server_traffic_usage").Create(map[string]interface{}{
						"server_id":      serverID,
						"year":           year,
						"month":          month,
						"upload_bytes":   int64(uploadBytes),
						"download_bytes": int64(downloadBytes),
					})
				}
			}

			if err != nil {
				facades.Log().Errorf("保存流量使用情况失败: %v", err)
				return err
			}
		}
	}

	facades.Log().Debugf("已保存服务器 %s 的网络信息", serverID)
	return nil
}

// SaveDiskIO 保存磁盘IO信息
func SaveDiskIO(serverID string, data map[string]interface{}) error {
	now := time.Now()
	timestamp := now.Unix()

	// 保存磁盘IO速度（转换为KB/s）
	if readSpeed, ok1 := data["read_speed"].(float64); ok1 {
		if writeSpeed, ok2 := data["write_speed"].(float64); ok2 {
			record := map[string]interface{}{
				"server_id":   serverID,
				"read_speed":  readSpeed / 1024,  // 转换为KB/s
				"write_speed": writeSpeed / 1024, // 转换为KB/s
				"timestamp":   now,
			}

			// 使用ORM的Create方法，自动处理timestamp字段
			err := facades.Orm().Query().Table("server_disk_io").Create(record)
			if err != nil {
				facades.Log().Errorf("保存磁盘IO失败: %v", err)
				return err
			}

			// 向前端推送磁盘IO实时数据点
			go func() {
				wsService := GetWebSocketService()
				readSpeedKB := readSpeed / 1024   // KB/s
				writeSpeedKB := writeSpeed / 1024 // KB/s
				realtimeDataPoint := map[string]interface{}{
					"type": "metrics_realtime",
					"data": map[string]interface{}{
						"server_id": serverID,
						"timestamp": timestamp,
						"metrics": map[string]interface{}{
							"disk_read":  FormatMetricValue(readSpeedKB),
							"disk_write": FormatMetricValue(writeSpeedKB),
						},
					},
				}
				wsService.BroadcastToFrontend(realtimeDataPoint)
			}()
		}
	}

	facades.Log().Debugf("已保存服务器 %s 的磁盘IO信息", serverID)
	return nil
}

// SaveVirtualMemory 保存虚拟内存信息
func SaveVirtualMemory(serverID string, data map[string]interface{}) error {
	timestamp := time.Now().Unix()

	record := map[string]interface{}{
		"server_id": serverID,
		"timestamp": timestamp,
	}

	if v, ok := data["virtual_memory_total"].(float64); ok {
		record["virtual_memory_total"] = int64(v)
	}
	if v, ok := data["virtual_memory_used"].(float64); ok {
		record["virtual_memory_used"] = int64(v)
	}
	if v, ok := data["virtual_memory_free"].(float64); ok {
		record["virtual_memory_free"] = int64(v)
	}

	// 使用ORM的Create方法，自动处理timestamp字段
	err := facades.Orm().Query().Table("server_virtual_memory").Create(record)
	if err != nil {
		facades.Log().Errorf("保存虚拟内存信息失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的虚拟内存信息", serverID)
	return nil
}

// ValidateAgentKey 验证agent key并返回server_id
func ValidateAgentKey(agentKey string) (string, error) {
	var server map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Where("agent_key", agentKey).
		First(&server)

	if err != nil {
		facades.Log().Channel("websocket").Warningf("验证agent key失败: %v", err)
		return "", errors.New("无效的agent key")
	}

	if server == nil {
		return "", errors.New("agent key不存在")
	}

	serverID, ok := server["id"].(string)
	if !ok {
		return "", errors.New("服务器ID格式错误")
	}

	return serverID, nil
}

// ValidateAgentAuth 验证agent key和IP地址并返回server_id
func ValidateAgentAuth(agentKey string, clientIP string) (string, error) {
	// 清理 IP 地址
	cleanIP := clientIP
	if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
		// 检查是否是 IPv6 地址格式 [::1]:port
		if strings.HasPrefix(clientIP, "[") {
			if endIdx := strings.Index(clientIP, "]:"); endIdx != -1 {
				cleanIP = clientIP[1:endIdx] // 提取 [::1] 中的 ::1
			}
		} else {
			// IPv4 格式 127.0.0.1:port，提取 IP 部分
			cleanIP = clientIP[:idx]
		}
	}

	keyPreview := agentKey
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8] + "..."
	}

	// 判断是否是本地回环地址
	isLocalhost := cleanIP == "127.0.0.1" || cleanIP == "::1" || cleanIP == "localhost"

	var servers []map[string]interface{}
	var err error

	if isLocalhost {
		// 如果是本地地址，只验证 agent_key，不验证 IP
		// 因为可能是通过代理、隧道或同一台机器连接
		facades.Log().Channel("websocket").Infof("检测到本地连接 (IP: %s)，仅验证 agent_key", cleanIP)
		err = facades.Orm().Query().Table("servers").
			Where("agent_key", agentKey).
			Get(&servers)
	} else {
		// 非本地地址，同时验证 agent_key 和 IP
		// 先尝试使用清理后的 IP 查询
		err = facades.Orm().Query().Table("servers").
			Where("agent_key", agentKey).
			Where("ip", cleanIP).
			Get(&servers)

		// 如果没有找到记录，尝试使用原始 IP
		if err == nil && len(servers) == 0 {
			facades.Log().Channel("websocket").Infof("使用清理后的IP查询无结果，尝试使用原始IP: cleanIP=%s, originalIP=%s", cleanIP, clientIP)
			err = facades.Orm().Query().Table("servers").
				Where("agent_key", agentKey).
				Where("ip", clientIP).
				Get(&servers)
		}
	}

	var server map[string]interface{}
	if err == nil && len(servers) > 0 {
		// 找到匹配的记录
		server = servers[0]
	} else if err == nil && len(servers) == 0 {
		// 未找到匹配记录
		err = errors.New("model value required")
	}

	if err != nil {
		// 记录认证失败信息
		facades.Log().Channel("websocket").Warningf("验证agent认证失败: %v (key=%s, cleanIP=%s, originalIP=%s)", err, keyPreview, cleanIP, clientIP)
		return "", errors.New("agent_key验证失败")
	}

	if server == nil {
		// 未找到匹配记录
		facades.Log().Channel("websocket").Warningf("未找到匹配的服务器记录 (key=%s, cleanIP=%s, originalIP=%s)", keyPreview, cleanIP, clientIP)
		return "", errors.New("agent_key验证失败")
	}

	serverID, ok := server["id"].(string)
	if !ok {
		return "", errors.New("服务器ID格式错误")
	}

	// 更新最后上报时间
	now := time.Now()
	facades.Orm().Query().Table("servers").
		Where("id", serverID).
		Update(map[string]interface{}{
			"last_report_time": now.Unix(),
			"updated_at":       now.Unix(),
		})

	return serverID, nil
}

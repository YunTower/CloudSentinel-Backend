package services

import (
	"errors"
	"time"

	"github.com/goravel/framework/facades"
)

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
	if v, ok := data["boot_time"].(string); ok {
		if bootTime, err := time.Parse(time.RFC3339, v); err == nil {
			updateData["boot_time"] = bootTime.Unix()
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
		"server_id":  serverID,
		"timestamp":  timestamp,
		"created_at": timestamp,
	}

	if v, ok := data["cpu_usage"].(float64); ok {
		metricsData["cpu_usage"] = v
	}
	if v, ok := data["memory_total"].(float64); ok {
		metricsData["memory_total"] = int64(v)
	}
	if v, ok := data["memory_used"].(float64); ok {
		metricsData["memory_used"] = int64(v)
	}
	if v, ok := data["memory_usage_percent"].(float64); ok {
		metricsData["memory_usage_percent"] = v
	}
	if v, ok := data["network_upload"].(float64); ok {
		metricsData["network_upload"] = v
	}
	if v, ok := data["network_download"].(float64); ok {
		metricsData["network_download"] = v
	}

	_, err = facades.Orm().Query().Exec("INSERT INTO server_metrics (server_id, timestamp, created_at, cpu_usage, memory_total, memory_used, memory_usage_percent, network_upload, network_download) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		metricsData["server_id"], metricsData["timestamp"], metricsData["created_at"],
		metricsData["cpu_usage"], metricsData["memory_total"], metricsData["memory_used"],
		metricsData["memory_usage_percent"], metricsData["network_upload"], metricsData["network_download"])
	if err != nil {
		facades.Log().Errorf("保存性能指标失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的性能指标", serverID)
	return nil
}

// SaveCPUInfo 保存CPU信息
func SaveCPUInfo(serverID string, data []interface{}) error {
	timestamp := time.Now().Unix()

	for _, item := range data {
		cpuData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		record := map[string]interface{}{
			"server_id": serverID,
			"timestamp": timestamp,
		}

		if v, ok := cpuData["cpu_name"].(string); ok {
			record["cpu_name"] = v
		}
		if v, ok := cpuData["cpu_usage"].(float64); ok {
			record["cpu_usage"] = v
		}
		if v, ok := cpuData["cores"].(float64); ok {
			record["cores"] = int(v)
		}

		_, err := facades.Orm().Query().Exec("INSERT INTO server_cpus (server_id, timestamp, cpu_name, cpu_usage, cores) VALUES (?, ?, ?, ?, ?)",
			record["server_id"], record["timestamp"], record["cpu_name"], record["cpu_usage"], record["cores"])
		if err != nil {
			facades.Log().Errorf("保存CPU信息失败: %v", err)
			return err
		}
	}

	facades.Log().Debugf("已保存服务器 %s 的CPU信息", serverID)
	return nil
}

// SaveMemoryInfo 保存内存信息
func SaveMemoryInfo(serverID string, data map[string]interface{}) error {
	timestamp := time.Now().Unix()

	record := map[string]interface{}{
		"server_id": serverID,
		"timestamp": timestamp,
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

	_, err := facades.Orm().Query().Exec("INSERT INTO server_memory_history (server_id, timestamp, memory_total, memory_used, memory_usage_percent) VALUES (?, ?, ?, ?, ?)",
		record["server_id"], record["timestamp"], record["memory_total"], record["memory_used"], record["memory_usage_percent"])
	if err != nil {
		facades.Log().Errorf("保存内存信息失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的内存信息", serverID)
	return nil
}

// SaveDiskInfo 保存磁盘信息
func SaveDiskInfo(serverID string, data []interface{}) error {
	timestamp := time.Now().Unix()

	// 先删除该服务器的旧磁盘记录（因为磁盘信息需要全量更新）
	facades.Orm().Query().Table("server_disks").
		Where("server_id", serverID).
		Delete()

	for _, item := range data {
		diskData, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		record := map[string]interface{}{
			"server_id":  serverID,
			"timestamp":  timestamp,
			"created_at": timestamp,
			"updated_at": timestamp,
		}

		if v, ok := diskData["mount_point"].(string); ok {
			record["mount_point"] = v
		}
		if v, ok := diskData["device"].(string); ok {
			record["device"] = v
		}
		if v, ok := diskData["total"].(float64); ok {
			record["total"] = int64(v)
		}
		if v, ok := diskData["used"].(float64); ok {
			record["used"] = int64(v)
		}
		if v, ok := diskData["free"].(float64); ok {
			record["free"] = int64(v)
		}
		if v, ok := diskData["usage_percent"].(float64); ok {
			record["usage_percent"] = v
		}

		_, err := facades.Orm().Query().Exec("INSERT INTO server_disks (server_id, timestamp, created_at, updated_at, mount_point, device, total, used, free, usage_percent) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			record["server_id"], record["timestamp"], record["created_at"], record["updated_at"],
			record["mount_point"], record["device"], record["total"], record["used"], record["free"], record["usage_percent"])
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
	timestamp := time.Now().Unix()

	// 保存TCP/UDP连接数
	if tcpConns, ok1 := data["tcp_connections"].(float64); ok1 {
		if udpConns, ok2 := data["udp_connections"].(float64); ok2 {
			record := map[string]interface{}{
				"server_id":       serverID,
				"tcp_connections": int(tcpConns),
				"udp_connections": int(udpConns),
				"timestamp":       timestamp,
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
				"timestamp":      timestamp,
			}

			_, err := facades.Orm().Query().Exec("INSERT INTO server_network_speed (server_id, upload_speed, download_speed, timestamp) VALUES (?, ?, ?, ?)",
				record["server_id"], record["upload_speed"], record["download_speed"], record["timestamp"])
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

			// 查询当前月份的流量记录
			var existing map[string]interface{}
			err := facades.Orm().Query().Table("server_traffic_usage").
				Where("server_id", serverID).
				Where("year", year).
				Where("month", month).
				First(&existing)

			if err == nil && existing != nil {
				// 更新现有记录
				_, err = facades.Orm().Query().Table("server_traffic_usage").
					Where("server_id", serverID).
					Where("year", year).
					Where("month", month).
					Update(map[string]interface{}{
						"upload_bytes":   int64(uploadBytes),
						"download_bytes": int64(downloadBytes),
						"updated_at":     timestamp,
					})
			} else {
				// 插入新记录
				_, err = facades.Orm().Query().Exec("INSERT INTO server_traffic_usage (server_id, year, month, upload_bytes, download_bytes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
					serverID, year, month, int64(uploadBytes), int64(downloadBytes), timestamp, timestamp)
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

	_, err := facades.Orm().Query().Exec("INSERT INTO server_virtual_memory (server_id, timestamp, virtual_memory_total, virtual_memory_used, virtual_memory_free) VALUES (?, ?, ?, ?, ?)",
		record["server_id"], record["timestamp"], record["virtual_memory_total"], record["virtual_memory_used"], record["virtual_memory_free"])
	if err != nil {
		facades.Log().Errorf("保存虚拟内存信息失败: %v", err)
		return err
	}

	facades.Log().Debugf("已保存服务器 %s 的虚拟内存信息", serverID)
	return nil
}

// ValidateAgentKey 验证agent key并返回server_id (保留兼容性)
func ValidateAgentKey(agentKey string) (string, error) {
	var server map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Where("agent_key", agentKey).
		First(&server)

	if err != nil {
		facades.Log().Errorf("验证agent key失败: %v", err)
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
	var server map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Where("agent_key", agentKey).
		Where("ip", clientIP).
		First(&server)

	if err != nil {
		facades.Log().Errorf("验证agent认证失败: %v (key=%s, ip=%s)", err, agentKey, clientIP)
		return "", errors.New("IP或agent_key验证失败")
	}

	if server == nil {
		facades.Log().Warning("未找到匹配的服务器记录 (key=" + agentKey + ", ip=" + clientIP + ")")
		return "", errors.New("IP或agent_key验证失败")
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


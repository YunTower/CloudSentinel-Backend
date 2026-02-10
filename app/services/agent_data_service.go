package services

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// SaveSystemInfo 保存系统信息
func SaveSystemInfo(serverID string, data map[string]interface{}) error {
	worker := GetGlobalDataWorker()
	worker.Enqueue(&saveSystemInfoJob{
		serverID: serverID,
		data:     data,
	})
	return nil
}

type saveSystemInfoJob struct {
	serverID string
	data     map[string]interface{}
}

func (j *saveSystemInfoJob) Execute() error {
	// 更新 servers 表
	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}

	if hostname, ok := j.data["hostname"].(string); ok {
		updates["name"] = hostname
	}
	if osInfo, ok := j.data["os"].(string); ok {
		updates["os_type"] = osInfo
	}
	if kernel, ok := j.data["kernel"].(string); ok {
		updates["kernel_version"] = kernel
	}
	if uptime, ok := j.data["uptime"].(float64); ok {
		updates["uptime"] = int64(uptime)
	}
	if bootTime, ok := j.data["boot_time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, bootTime); err == nil {
			updates["boot_time"] = t
		}
	}

	_, err := facades.Orm().Query().Model(&models.Server{}).Where("id = ?", j.serverID).Update(updates)
	return err
}

// SaveMetrics 保存性能指标
func SaveMetrics(serverID string, data map[string]interface{}) error {
	worker := GetGlobalDataWorker()
	worker.Enqueue(&saveMetricsJob{
		serverID: serverID,
		data:     data,
	})
	return nil
}

type saveMetricsJob struct {
	serverID string
	data     map[string]interface{}
}

func (j *saveMetricsJob) Execute() error {
	cpuUsage, _ := j.data["cpu_usage"].(float64)
	memoryUsage, _ := j.data["memory_usage"].(float64)
	diskUsage, _ := j.data["disk_usage"].(float64)

	// 网络速率
	netUp, _ := j.data["net_bytes_sent_rate"].(float64)
	netDown, _ := j.data["net_bytes_recv_rate"].(float64)

	metric := &models.ServerMetric{
		ServerID:        j.serverID,
		CPUUsage:        cpuUsage,
		MemoryUsage:     memoryUsage,
		DiskUsage:       diskUsage,
		NetworkUpload:   netUp,
		NetworkDownload: netDown,
		Timestamp:       time.Now(),
	}

	// 使用批量写入缓冲区代替直接写入数据库
	GetMetricBuffer().Enqueue(metric)
	return nil
}

// SaveAgentLogs 保存Agent日志
func SaveAgentLogs(serverID string, logs []interface{}) error {
	worker := GetGlobalDataWorker()
	worker.Enqueue(&saveAgentLogsJob{
		serverID: serverID,
		logs:     logs,
	})
	return nil
}

type saveAgentLogsJob struct {
	serverID string
	logs     []interface{}
}

func (j *saveAgentLogsJob) Execute() error {
	var logModels []models.AgentLog
	for _, logItem := range j.logs {
		l, ok := logItem.(map[string]interface{})
		if !ok {
			continue
		}

		level, _ := l["level"].(string)
		message, _ := l["message"].(string)

		var contextStr string
		if ctx, ok := l["context"]; ok {
			if ctxBytes, err := json.Marshal(ctx); err == nil {
				contextStr = string(ctxBytes)
			}
		}

		createdAt := time.Now()
		if timeStr, ok := l["time"].(string); ok {
			// Try parse time, assume RFC3339 or standard layout
			if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
				createdAt = t
			}
		}

		logModels = append(logModels, models.AgentLog{
			ServerID:  j.serverID,
			Level:     level,
			Message:   message,
			Context:   contextStr,
			CreatedAt: createdAt,
		})
	}

	if len(logModels) > 0 {
		if err := facades.Orm().Query().Create(&logModels); err != nil {
			facades.Log().Errorf("保存Agent日志失败: %v", err)
			return err
		}
	}
	return nil
}

// SaveMemoryInfo 保存内存信息
func SaveMemoryInfo(serverID string, data map[string]interface{}) error {
	// TODO: 实现保存逻辑
	return nil
}

// SaveDiskInfo 保存磁盘信息
func SaveDiskInfo(serverID string, data []interface{}) error {
	// TODO: 实现保存逻辑
	return nil
}

// SaveDiskIO 保存磁盘IO信息
func SaveDiskIO(serverID string, data map[string]interface{}) error {
	// TODO: 实现保存逻辑
	return nil
}

// SaveNetworkInfo 保存网络信息
func SaveNetworkInfo(serverID string, data map[string]interface{}) error {
	// TODO: 实现保存逻辑
	return nil
}

// SaveSwapInfo 保存Swap信息
func SaveSwapInfo(serverID string, data map[string]interface{}) error {
	// TODO: 实现保存逻辑
	return nil
}

// SaveProcessInfo 保存进程信息
func SaveProcessInfo(serverID string, data map[string]interface{}) error {
	worker := GetGlobalDataWorker()
	worker.Enqueue(&saveProcessInfoJob{
		serverID: serverID,
		data:     data,
	})
	return nil
}

type saveProcessInfoJob struct {
	serverID string
	data     map[string]interface{}
}

func (j *saveProcessInfoJob) Execute() error {
	// 更新 servers 表中的 service_status 字段
	_, err := facades.Orm().Query().Model(&models.Server{}).Where("id = ?", j.serverID).Update(map[string]interface{}{
		"service_status": j.data,
		"updated_at":     time.Now(),
	})
	return err
}

// SaveGPUInfo 保存GPU信息
func SaveGPUInfo(serverID string, data map[string]interface{}) error {
	worker := GetGlobalDataWorker()
	worker.Enqueue(&saveGPUInfoJob{
		serverID: serverID,
		data:     data,
	})
	return nil
}

type saveGPUInfoJob struct {
	serverID string
	data     map[string]interface{}
}

func (j *saveGPUInfoJob) Execute() error {
	// 更新 servers 表中的 gpu_info 字段
	_, err := facades.Orm().Query().Model(&models.Server{}).Where("id = ?", j.serverID).Update(map[string]interface{}{
		"gpu_info":   j.data,
		"updated_at": time.Now(),
	})
	return err
}

// CalculateUptime 计算运行时间
func CalculateUptime(input interface{}, _ ...interface{}) string {
	var uptime int64

	switch v := input.(type) {
	case int64:
		uptime = v
	case float64:
		uptime = int64(v)
	case *time.Time:
		if v != nil {
			uptime = int64(time.Since(*v).Seconds())
		}
	case time.Time:
		uptime = int64(time.Since(v).Seconds())
	default:
		return "0分"
	}

	days := uptime / 86400
	hours := (uptime % 86400) / 3600
	minutes := (uptime % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分", hours, minutes)
	}
	return fmt.Sprintf("%d分", minutes)
}

// FormatMetricValue 格式化指标值，保留2位小数
func FormatMetricValue(input interface{}) float64 {
	var value float64
	switch v := input.(type) {
	case float64:
		value = v
	case int:
		value = float64(v)
	case int64:
		value = float64(v)
	default:
		return 0.0
	}
	return math.Round(value*100) / 100
}

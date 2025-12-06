package services

import (
	"time"

	"goravel/app/repositories"

	"github.com/goravel/framework/facades"
)

type CleanupService struct {
	stopChan chan struct{}
}

func NewCleanupService() *CleanupService {
	return &CleanupService{
		stopChan: make(chan struct{}),
	}
}

// Start 启动数据清理服务
func (s *CleanupService) Start() {
	facades.Log().Info("数据清理服务已启动")

	// 每小时检查一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// 立即执行一次清理
	s.CleanupOldData()

	for {
		select {
		case <-ticker.C:
			s.CleanupOldData()
		case <-s.stopChan:
			facades.Log().Info("数据清理服务已停止")
			return
		}
	}
}

// Stop 停止数据清理服务
func (s *CleanupService) Stop() {
	close(s.stopChan)
}

// CleanupOldData 根据配置清理旧数据
func (s *CleanupService) CleanupOldData() {
	facades.Log().Info("开始清理旧数据...")

	// 从 system_settings 获取清理配置
	settingRepo := repositories.GetSystemSettingRepository()
	var configs []map[string]interface{}

	// 使用 GetJSONWithDefault 处理空值或无效 JSON 的情况
	defaultConfigs := []map[string]interface{}{}
	err := settingRepo.GetJSONWithDefault("log_cleanup_config", &configs, defaultConfigs)

	if err != nil {
		facades.Log().Errorf("获取清理配置失败: %v", err)
		return
	}

	if len(configs) == 0 {
		facades.Log().Info("没有启用的清理配置")
		return
	}

	// 表名映射：log_type -> 表名
	tableNameMap := map[string]string{
		"server_metrics":             "server_metrics",
		"server_memory_history":      "server_memory_history",
		"server_swap":                "server_swap",
		"server_network_connections": "server_network_connections",
		"server_network_speed":       "server_network_speed",
		"server_cpus":                "server_cpus",
		"alerts":                     "alerts",
		"service_monitor_alerts":     "service_monitor_alerts",
		"audit_logs":                 "audit_logs",
	}

	for _, config := range configs {
		// 检查是否启用
		enabled, ok := config["enabled"].(bool)
		if !ok {
			// 尝试从数字转换
			if enabledNum, ok := config["enabled"].(float64); ok {
				enabled = enabledNum == 1
			} else {
				continue
			}
		}
		if !enabled {
			continue
		}

		logType, ok := config["log_type"].(string)
		if !ok {
			continue
		}

		tableName, exists := tableNameMap[logType]
		if !exists {
			continue
		}

		// 获取保留天数
		var keepDays int
		if keepDaysFloat, ok := config["keep_days"].(float64); ok {
			keepDays = int(keepDaysFloat)
		} else if keepDaysInt, ok := config["keep_days"].(int); ok {
			keepDays = keepDaysInt
		} else {
			continue
		}

		// 计算截止时间
		cutoffTime := time.Now().AddDate(0, 0, -keepDays)
		cutoffTimestamp := cutoffTime.Unix()

		// 删除旧数据
		result, err := facades.Orm().Query().Table(tableName).
			Where("timestamp", "<", cutoffTimestamp).
			Delete()

		if err != nil {
			facades.Log().Errorf("清理表 %s 失败: %v", tableName, err)
			continue
		}

		rowsAffected := result.RowsAffected
		if rowsAffected > 0 {
			facades.Log().Infof("已清理表 %s 中 %d 条超过 %d 天的记录", tableName, rowsAffected, keepDays)
		}
	}

	facades.Log().Info("数据清理完成")
}

// CleanupTableData 清理指定表的数据
func (s *CleanupService) CleanupTableData(tableName string, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	cutoffTimestamp := cutoffTime.Unix()

	result, err := facades.Orm().Query().Table(tableName).
		Where("timestamp", "<", cutoffTimestamp).
		Delete()

	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected
	facades.Log().Infof("已清理表 %s 中 %d 条超过 %d 天的记录", tableName, rowsAffected, retentionDays)

	return nil
}

// CleanupServerMetrics 清理服务器性能指标
func (s *CleanupService) CleanupServerMetrics(retentionDays int) error {
	return s.CleanupTableData("server_metrics", retentionDays)
}

// CleanupServerCPUs 清理CPU信息
func (s *CleanupService) CleanupServerCPUs(retentionDays int) error {
	return s.CleanupTableData("server_cpus", retentionDays)
}

// CleanupServerMemoryHistory 清理内存历史
func (s *CleanupService) CleanupServerMemoryHistory(retentionDays int) error {
	return s.CleanupTableData("server_memory_history", retentionDays)
}

// CleanupServerNetworkConnections 清理网络连接历史
func (s *CleanupService) CleanupServerNetworkConnections(retentionDays int) error {
	return s.CleanupTableData("server_network_connections", retentionDays)
}

// CleanupServerNetworkSpeed 清理网络速度历史
func (s *CleanupService) CleanupServerNetworkSpeed(retentionDays int) error {
	return s.CleanupTableData("server_network_speed", retentionDays)
}

// CleanupServerSwap 清理Swap历史
func (s *CleanupService) CleanupServerSwap(retentionDays int) error {
	return s.CleanupTableData("server_swap", retentionDays)
}

// CleanupServerStatusLogs 清理服务器状态日志
func (s *CleanupService) CleanupServerStatusLogs(retentionDays int) error {
	return s.CleanupTableData("server_status_logs", retentionDays)
}

// CleanupServiceMonitorAlerts 清理服务监控告警
func (s *CleanupService) CleanupServiceMonitorAlerts(retentionDays int) error {
	return s.CleanupTableData("service_monitor_alerts", retentionDays)
}

// CleanupAlerts 清理告警记录
func (s *CleanupService) CleanupAlerts(retentionDays int) error {
	return s.CleanupTableData("alerts", retentionDays)
}

// OptimizeDatabase 优化数据库
func (s *CleanupService) OptimizeDatabase() error {
	facades.Log().Info("开始优化数据库...")

	// SQLite的优化命令
	_, err := facades.Orm().Query().Exec("VACUUM")
	if err != nil {
		facades.Log().Errorf("优化数据库失败: %v", err)
		return err
	}

	facades.Log().Info("数据库优化完成")
	return nil
}

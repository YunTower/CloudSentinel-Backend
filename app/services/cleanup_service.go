package services

import (
	"time"

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

	// 获取所有清理配置
	var configs []map[string]interface{}
	err := facades.Orm().Query().Table("log_cleanup_config").
		Where("enabled", 1).
		Get(&configs)

	if err != nil {
		facades.Log().Errorf("获取清理配置失败: %v", err)
		return
	}

	if len(configs) == 0 {
		facades.Log().Info("没有启用的清理配置")
		return
	}

	for _, config := range configs {
		tableName, ok := config["table_name"].(string)
		if !ok {
			continue
		}

		retentionDays, ok := config["retention_days"].(int64)
		if !ok {
			continue
		}

		// 计算截止时间
		cutoffTime := time.Now().AddDate(0, 0, -int(retentionDays))
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
			facades.Log().Infof("已清理表 %s 中 %d 条超过 %d 天的记录", tableName, rowsAffected, retentionDays)
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

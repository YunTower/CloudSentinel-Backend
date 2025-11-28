package repositories

import (
	"strings"
	"time"

	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// ServerMetricRepository 服务器指标
type ServerMetricRepository struct{}

// NewServerMetricRepository 创建服务器指标实例
func NewServerMetricRepository() *ServerMetricRepository {
	return &ServerMetricRepository{}
}

// GetLatestByServerID 获取服务器的最新指标
func (r *ServerMetricRepository) GetLatestByServerID(serverID string) (*models.ServerMetric, error) {
	var metric models.ServerMetric
	err := facades.Orm().Query().
		Where("server_id", serverID).
		OrderBy("timestamp", "desc").
		Limit(1).
		First(&metric)
	if err != nil {
		return nil, err
	}
	return &metric, nil
}

// GetLatestByServerIDs 批量获取多个服务器的最新指标
func (r *ServerMetricRepository) GetLatestByServerIDs(serverIDs []string) (map[string]*models.ServerMetric, error) {
	if len(serverIDs) == 0 {
		return make(map[string]*models.ServerMetric), nil
	}

	result := make(map[string]*models.ServerMetric)

	// 获取每个服务器的最新指标
	serverIDsInterface := stringsToInterfaceSlice(serverIDs)
	placeholders := strings.Repeat("?,", len(serverIDs)-1) + "?"

	sql := `SELECT server_id, cpu_usage, memory_usage, disk_usage, network_upload, network_download, uptime, timestamp
		FROM (
			SELECT server_id, cpu_usage, memory_usage, disk_usage, network_upload, network_download, uptime, timestamp,
				ROW_NUMBER() OVER (PARTITION BY server_id ORDER BY timestamp DESC) as rn
			FROM server_metrics
			WHERE server_id IN (` + placeholders + `)
		) WHERE rn = 1`

	var metrics []models.ServerMetric
	err := facades.Orm().Query().Raw(sql, serverIDsInterface...).Scan(&metrics)
	if err != nil {
		return nil, err
	}

	for i := range metrics {
		result[metrics[i].ServerID] = &metrics[i]
	}

	return result, nil
}

// Create 创建指标记录
func (r *ServerMetricRepository) Create(metric *models.ServerMetric) error {
	return facades.Orm().Query().Create(metric)
}

// GetHistory 获取历史指标数据
func (r *ServerMetricRepository) GetHistory(serverID string, startTime, endTime time.Time) ([]*models.ServerMetric, error) {
	var metrics []*models.ServerMetric
	err := facades.Orm().Query().
		Where("server_id", serverID).
		Where("timestamp", ">=", startTime).
		Where("timestamp", "<=", endTime).
		OrderBy("timestamp", "asc").
		Get(&metrics)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

// BatchCreate 批量创建指标记录
func (r *ServerMetricRepository) BatchCreate(metrics []*models.ServerMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	return facades.Orm().Query().Create(&metrics)
}

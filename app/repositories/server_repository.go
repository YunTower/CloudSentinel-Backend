package repositories

import (
	"encoding/json"

	"github.com/goravel/framework/contracts/database/orm"
	"goravel/app/models"

	"goravel/app/facades"
)

// serverCascadeTables 删除服务器时需要一并清理的关联表（均含 server_id 列）。
// 此前 agent_tasks / agent_logs / alert_states 三表缺失：
// agent_tasks 永久积压、agent_logs 无主、alert_states 残留会让告警状态机
// 对新服务器继承旧状态。
var serverCascadeTables = []string{
	"server_alert_rules",
	"server_notification_channels",
	"server_metrics",
	"server_disks",
	"server_status_logs",
	"server_cpus",
	"server_memory_history",
	"server_swap",
	"server_network_connections",
	"server_traffic_usage",
	"server_network_speed",
	"server_disk_io",
	"alerts",
	"service_monitor_rule_servers",
	"service_monitor_alerts",
	"agent_tasks",
	"agent_logs",
	"alert_states",
}

// ServerRepository 服务器
type ServerRepository struct{}

// NewServerRepository 创建服务器实例
func NewServerRepository() *ServerRepository {
	return &ServerRepository{}
}

// DeleteCascade 事务内删除服务器及其全部关联数据：
// - 逐表清理 server_id 关联数据（含 agent 任务/日志/告警状态）
// - 清理 server 来源的事件及其事件流水
// - 从所有服务监测的 server_ids 数组中移除该服务器，防止监测轮次
//   继续向已删除的 Agent 下发任务造成 agent_tasks 积压
func (r *ServerRepository) DeleteCascade(serverID string) error {
	return facades.Orm().Transaction(func(tx orm.Query) error {
		for _, table := range serverCascadeTables {
			if _, err := tx.Table(table).Where("server_id", serverID).Delete(); err != nil {
				return err
			}
		}
		// server 来源的事件：删除事件流水与事件本体
		incidents := []models.Incident{}
		if err := tx.Where("source_type", "server").Where("source_id", serverID).Get(&incidents); err != nil {
			return err
		}
		for i := range incidents {
			if _, err := tx.Where("incident_id", incidents[i].ID).Delete(&models.IncidentEvent{}); err != nil {
				return err
			}
		}
		if _, err := tx.Where("source_type", "server").Where("source_id", serverID).Delete(&models.Incident{}); err != nil {
			return err
		}
		// 从 service_monitors.server_ids（JSON 数组列）中移除。
		// 监测项数量有限，全量加载后在内存中精确过滤
		monitors := []models.ServiceMonitor{}
		if err := tx.Where("server_ids IS NOT NULL AND server_ids != ?", "[]").Get(&monitors); err != nil {
			return err
		}
		for i := range monitors {
			ids := monitors[i].ServerIDs
			filtered := make([]string, 0, len(ids))
			removed := false
			for _, sid := range ids {
				if sid == serverID {
					removed = true
					continue
				}
				filtered = append(filtered, sid)
			}
			if !removed {
				continue
			}
			encoded, err := json.Marshal(filtered)
			if err != nil {
				return err
			}
			if _, err := tx.Model(&models.ServiceMonitor{}).Where("id", monitors[i].ID).
				Update("server_ids", string(encoded)); err != nil {
				return err
			}
		}
		// 最后删除服务器本体
		if _, err := tx.Where("id", serverID).Delete(&models.Server{}); err != nil {
			return err
		}
		return nil
	})
}

// GetByID 根据ID获取服务器
func (r *ServerRepository) GetByID(id string) (*models.Server, error) {
	var server models.Server
	err := facades.Orm().Query().Where("id", id).First(&server)
	if err != nil {
		return nil, err
	}
	return &server, nil
}

// GetAll 获取所有服务器
func (r *ServerRepository) GetAll() ([]*models.Server, error) {
	var servers []*models.Server
	err := facades.Orm().Query().
		With("ServerGroup").
		OrderBy("created_at", "desc").
		Get(&servers)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

// GetOnline 获取所有在线服务器
func (r *ServerRepository) GetOnline() ([]*models.Server, error) {
	var servers []*models.Server
	err := facades.Orm().Query().Where("status", "online").Get(&servers)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

// GetWithMetrics 批量获取服务器及其最新指标
func (r *ServerRepository) GetWithMetrics(serverIDs []string) ([]*models.Server, error) {
	if len(serverIDs) == 0 {
		return []*models.Server{}, nil
	}

	var servers []*models.Server

	// 使用预加载获取指标
	err := facades.Orm().Query().
		WhereIn("id", stringsToInterfaceSlice(serverIDs)).
		With("ServerMetrics").
		Get(&servers)

	if err != nil {
		return nil, err
	}

	return servers, nil
}

// GetWithDisks 批量获取服务器及其磁盘信息
func (r *ServerRepository) GetWithDisks(serverIDs []string) ([]*models.Server, error) {
	if len(serverIDs) == 0 {
		return []*models.Server{}, nil
	}

	var servers []*models.Server

	err := facades.Orm().Query().
		WhereIn("id", stringsToInterfaceSlice(serverIDs)).
		With("ServerGroup").
		With("ServerDisks").
		With("ServerSwap").
		Get(&servers)

	if err != nil {
		return nil, err
	}

	return servers, nil
}

// GetWithMetricsAndDisks 批量获取服务器及其指标和磁盘信息
func (r *ServerRepository) GetWithMetricsAndDisks(serverIDs []string) ([]*models.Server, error) {
	if len(serverIDs) == 0 {
		return []*models.Server{}, nil
	}

	var servers []*models.Server

	// 使用预加载获取指标和磁盘信息
	err := facades.Orm().Query().
		WhereIn("id", stringsToInterfaceSlice(serverIDs)).
		With("ServerMetrics").
		With("ServerDisks").
		Get(&servers)

	if err != nil {
		return nil, err
	}

	return servers, nil
}

// UpdateStatus 更新服务器状态
func (r *ServerRepository) UpdateStatus(id string, status string) error {
	_, err := facades.Orm().Query().Model(&models.Server{}).Where("id", id).Update("status", status)
	return err
}

// Create 创建服务器
func (r *ServerRepository) Create(server *models.Server) error {
	return facades.Orm().Query().Create(server)
}

// Update 更新服务器
func (r *ServerRepository) Update(id string, data map[string]interface{}) error {
	_, err := facades.Orm().Query().Model(&models.Server{}).Where("id", id).Update(data)
	return err
}

// GetByIDWithRelations 根据ID获取服务器及其关联数据）
func (r *ServerRepository) GetByIDWithRelations(id string) (*models.Server, error) {
	var server models.Server
	err := facades.Orm().Query().
		Where("id", id).
		With("ServerGroup").
		With("ServerMetrics").
		With("ServerDisks").
		With("ServerMemoryHistory").
		With("ServerSwap").
		First(&server)

	if err != nil {
		return nil, err
	}
	return &server, nil
}

// GetByGroupID 根据分组ID获取服务器列表
func (r *ServerRepository) GetByGroupID(groupID uint) ([]*models.Server, error) {
	var servers []*models.Server
	err := facades.Orm().Query().
		Where("group_id", groupID).
		With("ServerGroup").
		OrderBy("created_at", "desc").
		Get(&servers)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

// GetWithoutGroup 获取未分组的服务器列表
func (r *ServerRepository) GetWithoutGroup() ([]*models.Server, error) {
	var servers []*models.Server
	err := facades.Orm().Query().
		Where("group_id", nil).
		With("ServerGroup").
		OrderBy("created_at", "desc").
		Get(&servers)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

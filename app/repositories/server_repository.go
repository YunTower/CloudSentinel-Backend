package repositories

import (
	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// ServerRepository 服务器
type ServerRepository struct{}

// NewServerRepository 创建服务器实例
func NewServerRepository() *ServerRepository {
	return &ServerRepository{}
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
	err := facades.Orm().Query().OrderBy("created_at", "desc").Get(&servers)
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
	serverIDsInterface := make([]interface{}, len(serverIDs))
	for i, id := range serverIDs {
		serverIDsInterface[i] = id
	}

	// 使用预加载获取指标
	err := facades.Orm().Query().
		WhereIn("id", serverIDsInterface).
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
	serverIDsInterface := make([]interface{}, len(serverIDs))
	for i, id := range serverIDs {
		serverIDsInterface[i] = id
	}

	err := facades.Orm().Query().
		WhereIn("id", serverIDsInterface).
		With("ServerDisks").
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
	serverIDsInterface := make([]interface{}, len(serverIDs))
	for i, id := range serverIDs {
		serverIDsInterface[i] = id
	}

	// 使用预加载获取指标和磁盘信息
	err := facades.Orm().Query().
		WhereIn("id", serverIDsInterface).
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

package repositories

import (
	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// ServerGroupRepository 服务器分组仓库
type ServerGroupRepository struct{}

// NewServerGroupRepository 创建服务器分组仓库实例
func NewServerGroupRepository() *ServerGroupRepository {
	return &ServerGroupRepository{}
}

// GetAll 获取所有分组
func (r *ServerGroupRepository) GetAll() ([]*models.ServerGroup, error) {
	var groups []*models.ServerGroup
	err := facades.Orm().Query().OrderBy("created_at", "desc").Get(&groups)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// GetByID 根据ID获取分组
func (r *ServerGroupRepository) GetByID(id uint) (*models.ServerGroup, error) {
	var group models.ServerGroup
	err := facades.Orm().Query().Where("id", id).First(&group)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// Create 创建分组
func (r *ServerGroupRepository) Create(group *models.ServerGroup) error {
	return facades.Orm().Query().Create(group)
}

// Update 更新分组
func (r *ServerGroupRepository) Update(group *models.ServerGroup) error {
	return facades.Orm().Query().Save(group)
}

// Delete 删除分组
func (r *ServerGroupRepository) Delete(id uint) error {
	// 先检查是否有服务器使用此分组，如果有则清空这些服务器的分组
	var servers []models.Server
	err := facades.Orm().Query().Table("servers").Where("group_id", id).Get(&servers)
	if err == nil && len(servers) > 0 {
		_, updateErr := facades.Orm().Query().Table("servers").Where("group_id", id).Update("group_id", nil)
		if updateErr != nil {
			return updateErr
		}
	}
	_, err = facades.Orm().Query().Where("id", id).Delete(&models.ServerGroup{})
	return err
}

// GetServersByGroupID 获取指定分组下的所有服务器
func (r *ServerGroupRepository) GetServersByGroupID(groupID uint) ([]*models.Server, error) {
	var servers []*models.Server
	err := facades.Orm().Query().Where("group_id", groupID).Get(&servers)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

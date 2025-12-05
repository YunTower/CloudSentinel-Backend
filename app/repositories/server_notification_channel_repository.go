package repositories

import (
	"fmt"

	"goravel/app/models"

	"github.com/goravel/framework/facades"
)

// ServerNotificationChannelRepository 服务器通知渠道 Repository
type ServerNotificationChannelRepository struct{}

// NewServerNotificationChannelRepository 创建服务器通知渠道 Repository 实例
func NewServerNotificationChannelRepository() *ServerNotificationChannelRepository {
	return &ServerNotificationChannelRepository{}
}

// GetByServerID 获取指定服务器的所有通知渠道配置
func (r *ServerNotificationChannelRepository) GetByServerID(serverID string) ([]*models.ServerNotificationChannel, error) {
	var channels []*models.ServerNotificationChannel
	err := facades.Orm().Query().Where("server_id", serverID).Get(&channels)
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// GetByServerIDAndType 根据服务器ID和通知类型获取配置
func (r *ServerNotificationChannelRepository) GetByServerIDAndType(serverID, notificationType string) (*models.ServerNotificationChannel, error) {
	var channel models.ServerNotificationChannel
	err := facades.Orm().Query().
		Where("server_id", serverID).
		Where("notification_type", notificationType).
		First(&channel)
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// CreateOrUpdate 创建或更新通知渠道配置
func (r *ServerNotificationChannelRepository) CreateOrUpdate(channel *models.ServerNotificationChannel) error {
	// 验证必要字段
	if channel.ServerID == "" {
		return fmt.Errorf("server_id cannot be empty")
	}
	if channel.NotificationType == "" {
		return fmt.Errorf("notification_type cannot be empty")
	}

	var existing models.ServerNotificationChannel
	err := facades.Orm().Query().
		Where("server_id", channel.ServerID).
		Where("notification_type", channel.NotificationType).
		First(&existing)

	// 检查是否是因为记录不存在而返回错误，或者 existing.ID 为 0（表示未找到）
	if err != nil || existing.ID == 0 {
		// 不存在则创建
		return facades.Orm().Query().Create(channel)
	}

	// 存在则更新
	existing.Enabled = channel.Enabled
	return facades.Orm().Query().Save(&existing)
}

// DeleteByServerID 删除指定服务器的所有通知渠道配置
func (r *ServerNotificationChannelRepository) DeleteByServerID(serverID string) error {
	_, err := facades.Orm().Query().
		Model(&models.ServerNotificationChannel{}).
		Where("server_id", serverID).
		Delete()
	return err
}

// DeleteByServerIDAndType 删除指定服务器和通知类型的配置
func (r *ServerNotificationChannelRepository) DeleteByServerIDAndType(serverID, notificationType string) error {
	_, err := facades.Orm().Query().
		Model(&models.ServerNotificationChannel{}).
		Where("server_id", serverID).
		Where("notification_type", notificationType).
		Delete()
	return err
}

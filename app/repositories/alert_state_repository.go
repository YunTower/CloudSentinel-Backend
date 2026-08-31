package repositories

import (
	"fmt"
	"time"

	"goravel/app/facades"
	"goravel/app/models"
)

// AlertStateRepository 告警状态持久化
type AlertStateRepository struct{}

// NewAlertStateRepository 创建告警状态仓库实例
func NewAlertStateRepository() *AlertStateRepository {
	return &AlertStateRepository{}
}

// Get 读取服务器+指标维度当前的告警状态；不存在时返回 nil。
// Goravel 的 First 在查无记录时返回 nil error 且 dest 保持零值，
// 因此用 ID==0 判定"不存在"；真实的 DB 错误会以 error 返回。
func (r *AlertStateRepository) Get(serverID, metric string) (*models.AlertState, error) {
	var row models.AlertState
	err := facades.Orm().Query().
		Where("server_id", serverID).
		Where("metric", metric).
		First(&row)
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row, nil
}

// DeleteByMetricPrefix 清理指定前缀的告警状态行（监测项删除时调用，
// 否则 service_monitor:<id>:* 行会成为永久孤儿）。
func (r *AlertStateRepository) DeleteByMetricPrefix(prefix string) error {
	_, err := facades.Orm().Query().Model(&models.AlertState{}).
		Where("metric LIKE ?", prefix+"%").
		Delete()
	return err
}

// Upsert 写入告警状态；lastNotifiedAt 非空时同时刷新最近通知时间。
// 该表是"重启不丢告警状态"的基础，任何写入失败都不能静默吞掉，
// 否则冷却期/状态会退化为进程内缓存行为。
func (r *AlertStateRepository) Upsert(serverID, metric, state string, lastNotifiedAt *time.Time) error {
	row, err := r.Get(serverID, metric)
	if err != nil {
		return fmt.Errorf("读取告警状态失败: %w", err)
	}
	now := time.Now()
	if row == nil {
		create := &models.AlertState{
			ServerID:       serverID,
			Metric:         metric,
			State:          state,
			LastNotifiedAt: lastNotifiedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := facades.Orm().Query().Create(create); err != nil {
			// 并发创建撞唯一索引时退化为更新，其余错误如实返回
			existing, getErr := r.Get(serverID, metric)
			if getErr != nil || existing == nil {
				return fmt.Errorf("创建告警状态失败: %w", err)
			}
			row = existing
		} else {
			return nil
		}
	}
	updates := map[string]interface{}{
		"state":      state,
		"updated_at": now,
	}
	if lastNotifiedAt != nil {
		updates["last_notified_at"] = *lastNotifiedAt
	}
	_, err = facades.Orm().Query().Model(&models.AlertState{}).Where("id", row.ID).Update(updates)
	if err != nil {
		return fmt.Errorf("更新告警状态失败: %w", err)
	}
	return nil
}

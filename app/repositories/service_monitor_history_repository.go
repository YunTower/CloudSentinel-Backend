package repositories

import (
	"goravel/app/models"
	"sync"
	"time"

	"goravel/app/facades"
)

var (
	serviceMonitorHistoryRepoOnce     sync.Once
	serviceMonitorHistoryRepoInstance *ServiceMonitorHistoryRepository
)

type ServiceMonitorHistoryRepository struct{}

func GetServiceMonitorHistoryRepository() *ServiceMonitorHistoryRepository {
	serviceMonitorHistoryRepoOnce.Do(func() {
		serviceMonitorHistoryRepoInstance = &ServiceMonitorHistoryRepository{}
	})
	return serviceMonitorHistoryRepoInstance
}

// Save inserts a new history record.
func (r *ServiceMonitorHistoryRepository) Save(h *models.ServiceMonitorHistory) error {
	return facades.Orm().Query().Create(h)
}

// GetLast returns the last `limit` history entries for a monitor, newest first.
func (r *ServiceMonitorHistoryRepository) GetLast(monitorID uint, limit int) ([]*models.ServiceMonitorHistory, error) {
	var history []*models.ServiceMonitorHistory
	err := facades.Orm().Query().
		Where("monitor_id", monitorID).
		OrderBy("checked_at", "desc").
		Limit(limit).
		Get(&history)
	return history, err
}

// GetBatchLast fetches the last `limit` history entries for multiple monitors at once.
// Returns a map keyed by monitorID, with entries ordered oldest-first (for chronological display).
func (r *ServiceMonitorHistoryRepository) GetBatchLast(monitorIDs []uint, limit int) (map[uint][]*models.ServiceMonitorHistory, error) {
	if len(monitorIDs) == 0 {
		return map[uint][]*models.ServiceMonitorHistory{}, nil
	}

	result := make(map[uint][]*models.ServiceMonitorHistory, len(monitorIDs))
	for _, id := range monitorIDs {
		entries, err := r.GetLast(id, limit)
		if err != nil {
			return nil, err
		}
		// Reverse so the display order is oldest → newest (left to right)
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
		result[id] = entries
	}
	return result, nil
}

// PruneOld deletes history records beyond the most recent `keepCount` for a monitor.
func (r *ServiceMonitorHistoryRepository) PruneOld(monitorID uint, keepCount int) {
	// Find the checked_at of the (keepCount+1)-th newest record and delete anything older.
	var threshold models.ServiceMonitorHistory
	err := facades.Orm().Query().
		Where("monitor_id", monitorID).
		OrderBy("checked_at", "desc").
		Offset(keepCount).
		Limit(1).
		First(&threshold)
	if err != nil {
		return // fewer than keepCount rows – nothing to prune
	}
	facades.Orm().Query().
		Where("monitor_id = ? AND checked_at <= ?", monitorID, threshold.CheckedAt).
		Delete(&models.ServiceMonitorHistory{})
}

// PruneBefore deletes history entries older than the given cutoff for a monitor.
func (r *ServiceMonitorHistoryRepository) PruneBefore(monitorID uint, cutoff time.Time) {
	facades.Orm().Query().
		Where("monitor_id = ? AND checked_at < ?", monitorID, cutoff).
		Delete(&models.ServiceMonitorHistory{})
}

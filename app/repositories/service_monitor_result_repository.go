package repositories

import (
	"goravel/app/models"
	"strings"
	"time"

	"github.com/goravel/framework/facades"
)

type ServiceMonitorResultRepository struct{}

func NewServiceMonitorResultRepository() *ServiceMonitorResultRepository {
	return &ServiceMonitorResultRepository{}
}

func (r *ServiceMonitorResultRepository) Save(result *models.ServiceMonitorResult) error {
	return facades.Orm().Query().Create(result)
}

func (r *ServiceMonitorResultRepository) GetLast(monitorID uint, limit int) ([]*models.ServiceMonitorResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var results []*models.ServiceMonitorResult
	err := facades.Orm().Query().
		Where("monitor_id", monitorID).
		OrderBy("checked_at", "desc").
		Limit(limit).
		Get(&results)
	return results, err
}

func (r *ServiceMonitorResultRepository) PruneBefore(monitorID uint, cutoff time.Time) {
	facades.Orm().Query().
		Where("monitor_id = ? AND checked_at < ?", monitorID, cutoff).
		Delete(&models.ServiceMonitorResult{})
}

func (r *ServiceMonitorResultRepository) BatchUptimeStats(monitorIDs []uint, since time.Time) (map[uint]models.UptimeStat, error) {
	stats := make(map[uint]models.UptimeStat, len(monitorIDs))
	if len(monitorIDs) == 0 {
		return stats, nil
	}

	args := make([]interface{}, 0, len(monitorIDs)+1)
	placeholders := make([]string, 0, len(monitorIDs))
	for _, id := range monitorIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	args = append(args, since)

	sql := `SELECT
			monitor_id,
			COUNT(*) AS total_checks,
			SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) AS up_checks,
			SUM(CASE WHEN status = 'slow' THEN 1 ELSE 0 END) AS slow_checks,
			SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END) AS down_checks,
			COALESCE(AVG(CASE WHEN response_time > 0 THEN response_time ELSE NULL END), 0) AS avg_response_time
		FROM service_monitor_results
		WHERE monitor_id IN (` + strings.Join(placeholders, ",") + `)
			AND checked_at >= ?
		GROUP BY monitor_id`

	var rows []struct {
		MonitorID       uint    `gorm:"column:monitor_id"`
		TotalChecks     int     `gorm:"column:total_checks"`
		UpChecks        int     `gorm:"column:up_checks"`
		SlowChecks      int     `gorm:"column:slow_checks"`
		DownChecks      int     `gorm:"column:down_checks"`
		AvgResponseTime float64 `gorm:"column:avg_response_time"`
	}
	if err := facades.Orm().Query().Raw(sql, args...).Scan(&rows); err != nil {
		return nil, err
	}

	for _, row := range rows {
		rate := 0.0
		if row.TotalChecks > 0 {
			rate = float64(row.UpChecks) / float64(row.TotalChecks) * 100
		}
		stats[row.MonitorID] = models.UptimeStat{
			TotalChecks:     row.TotalChecks,
			UpChecks:        row.UpChecks,
			SlowChecks:      row.SlowChecks,
			DownChecks:      row.DownChecks,
			UptimeRate:      roundFloat(rate, 3),
			AvgResponseTime: roundFloat(row.AvgResponseTime, 1),
		}
	}

	return stats, nil
}

func roundFloat(value float64, precision int) float64 {
	if precision <= 0 {
		if value >= 0 {
			return float64(int(value + 0.5))
		}
		return float64(int(value - 0.5))
	}
	multiplier := 1.0
	for i := 0; i < precision; i++ {
		multiplier *= 10
	}
	if value >= 0 {
		return float64(int(value*multiplier+0.5)) / multiplier
	}
	return float64(int(value*multiplier-0.5)) / multiplier
}

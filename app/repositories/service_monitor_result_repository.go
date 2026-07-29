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

// DailyStatusBar is one day in a Polar-style uptime timeline.
type DailyStatusBar struct {
	Status       string    `json:"status"` // up, slow, down, or empty when no data
	ResponseTime int       `json:"response_time"`
	CheckedAt    time.Time `json:"checked_at"`
	TotalChecks  int       `json:"total_checks"`
}

// BatchDailyStatus returns a fixed-length daily timeline (oldest → newest) for each monitor.
// Day status priority: down > slow > up. Empty days keep status "".
func (r *ServiceMonitorResultRepository) BatchDailyStatus(monitorIDs []uint, days int) (map[uint][]DailyStatusBar, error) {
	out := make(map[uint][]DailyStatusBar, len(monitorIDs))
	if len(monitorIDs) == 0 {
		return out, nil
	}
	if days <= 0 {
		days = 30
	}

	now := time.Now()
	// Align to local midnight so "today" is the last slot.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := today.AddDate(0, 0, -(days - 1))

	for _, id := range monitorIDs {
		bars := make([]DailyStatusBar, days)
		for i := 0; i < days; i++ {
			day := since.AddDate(0, 0, i)
			bars[i] = DailyStatusBar{CheckedAt: day}
		}
		out[id] = bars
	}

	args := make([]interface{}, 0, len(monitorIDs)+1)
	placeholders := make([]string, 0, len(monitorIDs))
	for _, id := range monitorIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	args = append(args, since)

	// SQLite: date(); MySQL/Postgres also accept date()/DATE() in common setups.
	sql := `SELECT
			monitor_id,
			date(checked_at) AS day,
			SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END) AS down_checks,
			SUM(CASE WHEN status = 'slow' THEN 1 ELSE 0 END) AS slow_checks,
			SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) AS up_checks,
			COUNT(*) AS total_checks,
			COALESCE(AVG(CASE WHEN response_time > 0 THEN response_time ELSE NULL END), 0) AS avg_response_time
		FROM service_monitor_results
		WHERE monitor_id IN (` + strings.Join(placeholders, ",") + `)
			AND checked_at >= ?
		GROUP BY monitor_id, date(checked_at)`

	var rows []struct {
		MonitorID       uint    `gorm:"column:monitor_id"`
		Day             string  `gorm:"column:day"`
		DownChecks      int     `gorm:"column:down_checks"`
		SlowChecks      int     `gorm:"column:slow_checks"`
		UpChecks        int     `gorm:"column:up_checks"`
		TotalChecks     int     `gorm:"column:total_checks"`
		AvgResponseTime float64 `gorm:"column:avg_response_time"`
	}
	if err := facades.Orm().Query().Raw(sql, args...).Scan(&rows); err != nil {
		return nil, err
	}

	for _, row := range rows {
		bars, ok := out[row.MonitorID]
		if !ok {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", row.Day, now.Location())
		if err != nil {
			continue
		}
		idx := int(day.Sub(since).Hours() / 24)
		if idx < 0 || idx >= days {
			continue
		}
		status := ""
		if row.DownChecks > 0 {
			status = "down"
		} else if row.SlowChecks > 0 {
			status = "slow"
		} else if row.UpChecks > 0 {
			status = "up"
		}
		bars[idx] = DailyStatusBar{
			Status:       status,
			ResponseTime: int(row.AvgResponseTime + 0.5),
			CheckedAt:    day,
			TotalChecks:  row.TotalChecks,
		}
	}

	return out, nil
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

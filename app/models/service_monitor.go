package models

import "time"

type ServiceMonitor struct {
	ID           uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name         string     `gorm:"column:name;not null" json:"name"`
	Type         string     `gorm:"column:type;not null" json:"type"` // http, https, tcp, udp
	Target       string     `gorm:"column:target;not null" json:"target"`
	GroupName    string     `gorm:"column:group_name" json:"group_name"`
	Port         int        `gorm:"column:port;default:0" json:"port"`
	Interval     int        `gorm:"column:interval;default:60" json:"interval"` // seconds
	Timeout      int        `gorm:"column:timeout;default:10" json:"timeout"`   // seconds
	Enabled      bool       `gorm:"column:enabled;default:1" json:"enabled"`
	Status       string     `gorm:"column:status;default:unknown" json:"status"` // up, down, unknown
	LastCheckAt  *time.Time `gorm:"column:last_check_at" json:"last_check_at"`
	ResponseTime int        `gorm:"column:response_time;default:0" json:"response_time"` // ms
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`

	// Server IDs that perform the check (stored as JSON)
	ServerIDs            []string `gorm:"column:server_ids;serializer:json" json:"server_ids"`
	ExpectStatus         int      `gorm:"column:expect_status;default:0" json:"expect_status"` // 0 = any 2xx
	ExpectBody           string   `gorm:"column:expect_body" json:"expect_body"`               // substring match, empty = skip
	HTTPMethod           string   `gorm:"column:http_method;default:GET" json:"http_method"`
	HTTPHeaders          string   `gorm:"column:http_headers" json:"http_headers"`
	HTTPBody             string   `gorm:"column:http_body" json:"http_body"`
	FailureThreshold     int      `gorm:"column:failure_threshold;default:1" json:"failure_threshold"`
	RecoveryThreshold    int      `gorm:"column:recovery_threshold;default:1" json:"recovery_threshold"`
	ConsecutiveFailures  int      `gorm:"column:consecutive_failures;default:0" json:"consecutive_failures"`
	ConsecutiveSuccesses int      `gorm:"column:consecutive_successes;default:0" json:"consecutive_successes"`

	// HTTPS 证书有效期检测（仅 type=https 且开启时生效）
	CheckCertExpiry bool       `gorm:"column:check_cert_expiry;default:0" json:"check_cert_expiry"`
	CertExpiresAt   *time.Time `gorm:"column:cert_expires_at" json:"cert_expires_at"`
	CertDaysLeft    *int       `gorm:"column:cert_days_left" json:"cert_days_left"`

	// Virtual field – not stored in DB, populated by GetAll
	History []*ServiceMonitorHistory `gorm:"-" json:"history"`
	Uptime  map[string]UptimeStat    `gorm:"-" json:"uptime,omitempty"`
}

func (s *ServiceMonitor) TableName() string {
	return "service_monitors"
}

type UptimeStat struct {
	TotalChecks     int     `json:"total_checks"`
	UpChecks        int     `json:"up_checks"`
	SlowChecks      int     `json:"slow_checks"`
	DownChecks      int     `json:"down_checks"`
	UptimeRate      float64 `json:"uptime_rate"`
	AvgResponseTime float64 `json:"avg_response_time"`
}

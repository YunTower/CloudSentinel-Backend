package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// Incident represents one service/server problem or maintenance window.
type Incident struct {
	ID          uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SourceType  string     `gorm:"column:source_type;not null;size:50;index" json:"source_type"`
	SourceID    string     `gorm:"column:source_id;not null;size:100;index" json:"source_id"`
	Title       string     `gorm:"column:title;not null;size:255" json:"title"`
	Status      string     `gorm:"column:status;not null;size:30;index" json:"status"` // active, resolved
	Impact      string     `gorm:"column:impact;not null;size:30" json:"impact"`       // degraded, outage, maintenance
	StartedAt   time.Time  `gorm:"column:started_at;not null;index" json:"started_at"`
	ResolvedAt  *time.Time `gorm:"column:resolved_at" json:"resolved_at"`
	LastEventAt time.Time  `gorm:"column:last_event_at;not null;index" json:"last_event_at"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`

	Events []*IncidentEvent `gorm:"foreignKey:IncidentID;references:ID" json:"events,omitempty"`

	orm.Model
}

func (i *Incident) TableName() string {
	return "incidents"
}

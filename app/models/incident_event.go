package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// IncidentEvent is one timeline entry inside an incident.
type IncidentEvent struct {
	ID         uint      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	IncidentID uint      `gorm:"column:incident_id;not null;index" json:"incident_id"`
	EventType  string    `gorm:"column:event_type;not null;size:50;index" json:"event_type"` // opened, update, resolved
	Status     string    `gorm:"column:status;not null;size:50" json:"status"`
	Message    string    `gorm:"column:message;type:text;not null" json:"message"`
	Metadata   string    `gorm:"column:metadata;type:text" json:"metadata,omitempty"`
	OccurredAt time.Time `gorm:"column:occurred_at;not null;index" json:"occurred_at"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`

	orm.Model
}

func (i *IncidentEvent) TableName() string {
	return "incident_events"
}

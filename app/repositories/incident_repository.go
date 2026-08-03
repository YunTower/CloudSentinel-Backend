package repositories

import (
	"goravel/app/models"
	"time"

	"goravel/app/facades"
)

type IncidentRepository struct{}

func NewIncidentRepository() *IncidentRepository {
	return &IncidentRepository{}
}

func (r *IncidentRepository) GetOpenBySource(sourceType, sourceID string) (*models.Incident, error) {
	var incident models.Incident
	err := facades.Orm().Query().
		Where("source_type = ? AND source_id = ? AND status = ?", sourceType, sourceID, "active").
		OrderBy("started_at", "desc").
		First(&incident)
	if err != nil {
		return nil, err
	}
	return &incident, nil
}

func (r *IncidentRepository) Create(incident *models.Incident) error {
	return facades.Orm().Query().Create(incident)
}

func (r *IncidentRepository) GetByID(id uint) (*models.Incident, error) {
	var incident models.Incident
	err := facades.Orm().Query().
		With("Events").
		Where("id", id).
		First(&incident)
	if err != nil {
		return nil, err
	}
	return &incident, nil
}

func (r *IncidentRepository) Update(id uint, data map[string]interface{}) error {
	_, err := facades.Orm().Query().Model(&models.Incident{}).Where("id", id).Update(data)
	return err
}

func (r *IncidentRepository) AddEvent(event *models.IncidentEvent) error {
	return facades.Orm().Query().Create(event)
}

func (r *IncidentRepository) List(limit int) ([]*models.Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var incidents []*models.Incident
	err := facades.Orm().Query().
		With("Events").
		OrderBy("last_event_at", "desc").
		Limit(limit).
		Get(&incidents)
	return incidents, err
}

func (r *IncidentRepository) ListSince(since time.Time, limit int) ([]*models.Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var incidents []*models.Incident
	err := facades.Orm().Query().
		With("Events").
		Where("last_event_at >= ?", since).
		OrderBy("last_event_at", "desc").
		Limit(limit).
		Get(&incidents)
	return incidents, err
}

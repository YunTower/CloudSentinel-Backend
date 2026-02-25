package repositories

import (
	"goravel/app/models"
	"sync"

	"github.com/goravel/framework/facades"
)

var (
	serviceMonitorRepoOnce     sync.Once
	serviceMonitorRepoInstance *ServiceMonitorRepository
)

type ServiceMonitorRepository struct{}

func GetServiceMonitorRepository() *ServiceMonitorRepository {
	serviceMonitorRepoOnce.Do(func() {
		serviceMonitorRepoInstance = &ServiceMonitorRepository{}
	})
	return serviceMonitorRepoInstance
}

func (r *ServiceMonitorRepository) GetAll() ([]*models.ServiceMonitor, error) {
	var monitors []*models.ServiceMonitor
	err := facades.Orm().Query().OrderBy("created_at", "desc").Get(&monitors)
	return monitors, err
}

func (r *ServiceMonitorRepository) GetByID(id uint) (*models.ServiceMonitor, error) {
	var monitor models.ServiceMonitor
	err := facades.Orm().Query().Where("id", id).First(&monitor)
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func (r *ServiceMonitorRepository) GetEnabled() ([]*models.ServiceMonitor, error) {
	var monitors []*models.ServiceMonitor
	err := facades.Orm().Query().Where("enabled", true).Get(&monitors)
	return monitors, err
}

func (r *ServiceMonitorRepository) Create(m *models.ServiceMonitor) error {
	return facades.Orm().Query().Create(m)
}

func (r *ServiceMonitorRepository) Update(id uint, data map[string]interface{}) error {
	_, err := facades.Orm().Query().Model(&models.ServiceMonitor{}).Where("id", id).Update(data)
	return err
}

func (r *ServiceMonitorRepository) Delete(id uint) error {
	_, err := facades.Orm().Query().Where("id", id).Delete(&models.ServiceMonitor{})
	return err
}

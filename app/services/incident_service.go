package services

import (
	"encoding/json"
	"fmt"
	"goravel/app/models"
	"goravel/app/repositories"
	"time"

	"github.com/goravel/framework/facades"
)

type IncidentService struct {
	repo *repositories.IncidentRepository
}

func NewIncidentService() *IncidentService {
	return &IncidentService{repo: repositories.NewIncidentRepository()}
}

func (s *IncidentService) OpenOrUpdateServiceMonitorIncident(monitorID uint, status string, responseTime int, cause error) {
	monitor, err := repositories.GetServiceMonitorRepository().GetByID(monitorID)
	if err != nil || monitor == nil {
		return
	}

	sourceID := fmt.Sprintf("%d", monitorID)
	now := time.Now()
	impact := "outage"
	if status == "slow" {
		impact = "degraded"
	}
	title := fmt.Sprintf("%s 服务异常", monitor.Name)

	incident, err := s.repo.GetOpenBySource("service_monitor", sourceID)
	if err != nil || incident == nil {
		incident = &models.Incident{
			SourceType:  "service_monitor",
			SourceID:    sourceID,
			Title:       title,
			Status:      "active",
			Impact:      impact,
			StartedAt:   now,
			LastEventAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.repo.Create(incident); err != nil {
			facades.Log().Warningf("创建服务监测事件失败: monitor_id=%d, error=%v", monitorID, err)
			return
		}
		s.addServiceMonitorEvent(incident.ID, "opened", status, monitor, responseTime, cause, now)
		return
	}

	_ = s.repo.Update(incident.ID, map[string]interface{}{
		"impact":        impact,
		"last_event_at": now,
		"updated_at":    now,
	})
	s.addServiceMonitorEvent(incident.ID, "update", status, monitor, responseTime, cause, now)
}

func (s *IncidentService) ResolveServiceMonitorIncident(monitorID uint, previousStatus string, responseTime int) {
	monitor, err := repositories.GetServiceMonitorRepository().GetByID(monitorID)
	if err != nil || monitor == nil {
		return
	}

	sourceID := fmt.Sprintf("%d", monitorID)
	incident, err := s.repo.GetOpenBySource("service_monitor", sourceID)
	if err != nil || incident == nil {
		return
	}

	now := time.Now()
	_ = s.repo.Update(incident.ID, map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   now,
		"last_event_at": now,
		"updated_at":    now,
	})
	s.addServiceMonitorEvent(incident.ID, "resolved", "up", monitor, responseTime, fmt.Errorf("previous status: %s", previousStatus), now)
}

func (s *IncidentService) OpenServerOfflineIncident(serverID string) {
	server, err := repositories.GetServerRepository().GetByID(serverID)
	if err != nil || server == nil {
		return
	}

	now := time.Now()
	title := fmt.Sprintf("%s 服务器离线", server.Name)
	incident, err := s.repo.GetOpenBySource("server", serverID)
	if err != nil || incident == nil {
		incident = &models.Incident{
			SourceType:  "server",
			SourceID:    serverID,
			Title:       title,
			Status:      "active",
			Impact:      "outage",
			StartedAt:   now,
			LastEventAt: now,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.repo.Create(incident); err != nil {
			facades.Log().Warningf("创建服务器离线事件失败: server_id=%s, error=%v", serverID, err)
			return
		}
		s.addServerEvent(incident.ID, "opened", "offline", server, now)
		return
	}

	_ = s.repo.Update(incident.ID, map[string]interface{}{
		"impact":        "outage",
		"last_event_at": now,
		"updated_at":    now,
	})
	s.addServerEvent(incident.ID, "update", "offline", server, now)
}

func (s *IncidentService) ResolveServerOfflineIncident(serverID string) {
	server, err := repositories.GetServerRepository().GetByID(serverID)
	if err != nil || server == nil {
		return
	}

	incident, err := s.repo.GetOpenBySource("server", serverID)
	if err != nil || incident == nil {
		return
	}

	now := time.Now()
	_ = s.repo.Update(incident.ID, map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   now,
		"last_event_at": now,
		"updated_at":    now,
	})
	s.addServerEvent(incident.ID, "resolved", "online", server, now)
}

func (s *IncidentService) addServiceMonitorEvent(incidentID uint, eventType string, status string, monitor *models.ServiceMonitor, responseTime int, cause error, occurredAt time.Time) {
	statusLabel := map[string]string{
		"up":   "正常",
		"down": "故障",
		"slow": "响应慢",
	}[status]
	if statusLabel == "" {
		statusLabel = status
	}

	message := fmt.Sprintf("服务 %s 当前状态：%s，响应时间 %dms。", monitor.Name, statusLabel, responseTime)
	if cause != nil {
		message += fmt.Sprintf(" %v", cause)
	}

	metadata, _ := json.Marshal(map[string]interface{}{
		"monitor_id":    monitor.ID,
		"monitor_name":  monitor.Name,
		"type":          monitor.Type,
		"target":        monitor.Target,
		"status":        status,
		"response_time": responseTime,
	})

	if err := s.repo.AddEvent(&models.IncidentEvent{
		IncidentID: incidentID,
		EventType:  eventType,
		Status:     status,
		Message:    message,
		Metadata:   string(metadata),
		OccurredAt: occurredAt,
		CreatedAt:  occurredAt,
		UpdatedAt:  occurredAt,
	}); err != nil {
		facades.Log().Warningf("写入服务监测事件时间线失败: incident_id=%d, error=%v", incidentID, err)
	}
}

func (s *IncidentService) addServerEvent(incidentID uint, eventType string, status string, server *models.Server, occurredAt time.Time) {
	statusLabel := map[string]string{
		"online":  "在线",
		"offline": "离线",
	}[status]
	if statusLabel == "" {
		statusLabel = status
	}

	message := fmt.Sprintf("服务器 %s 当前状态：%s。", server.Name, statusLabel)
	metadata, _ := json.Marshal(map[string]interface{}{
		"server_id":   server.ID,
		"server_name": server.Name,
		"ip":          server.IP,
		"status":      status,
	})

	if err := s.repo.AddEvent(&models.IncidentEvent{
		IncidentID: incidentID,
		EventType:  eventType,
		Status:     status,
		Message:    message,
		Metadata:   string(metadata),
		OccurredAt: occurredAt,
		CreatedAt:  occurredAt,
		UpdatedAt:  occurredAt,
	}); err != nil {
		facades.Log().Warningf("写入服务器事件时间线失败: incident_id=%d, error=%v", incidentID, err)
	}
}

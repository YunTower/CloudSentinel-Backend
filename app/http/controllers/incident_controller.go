package controllers

import (
	"goravel/app/models"
	"goravel/app/repositories"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
)

type IncidentController struct{}

func NewIncidentController() *IncidentController {
	return &IncidentController{}
}

func (c *IncidentController) GetAll(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	incidents, err := repositories.NewIncidentRepository().List(100)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   incidents,
	})
}

func (c *IncidentController) CreateMaintenance(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	var req struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "参数错误"})
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	if req.Title == "" || req.Message == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "title 和 message 不能为空"})
	}
	if len(req.Title) > 255 || len(req.Message) > 5000 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "内容过长"})
	}

	now := time.Now()
	repo := repositories.NewIncidentRepository()
	incident := &models.Incident{
		SourceType:  "maintenance",
		SourceID:    "manual",
		Title:       req.Title,
		Status:      "active",
		Impact:      "maintenance",
		StartedAt:   now,
		LastEventAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(incident); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": err.Error()})
	}
	if err := repo.AddEvent(&models.IncidentEvent{
		IncidentID: incident.ID,
		EventType:  "opened",
		Status:     "maintenance",
		Message:    req.Message,
		OccurredAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": err.Error()})
	}
	created, _ := repo.GetByID(incident.ID)
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{"status": true, "data": created})
}

func (c *IncidentController) AddMaintenanceUpdate(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	id, err := parseIncidentRouteID(ctx)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "无效ID"})
	}
	var req struct {
		Message string `json:"message"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "参数错误"})
	}
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" || len(req.Message) > 5000 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "message 不能为空且不能超过 5000 字"})
	}

	repo := repositories.NewIncidentRepository()
	incident, err := repo.GetByID(id)
	if err != nil || incident.SourceType != "maintenance" || incident.Status != "active" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "只能更新进行中的维护事件"})
	}
	now := time.Now()
	if err := repo.AddEvent(&models.IncidentEvent{
		IncidentID: id,
		EventType:  "update",
		Status:     "maintenance",
		Message:    req.Message,
		OccurredAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": err.Error()})
	}
	_ = repo.Update(id, map[string]interface{}{"last_event_at": now, "updated_at": now})
	updated, _ := repo.GetByID(id)
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{"status": true, "data": updated})
}

func (c *IncidentController) ResolveMaintenance(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	id, err := parseIncidentRouteID(ctx)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "无效ID"})
	}
	var req struct {
		Message string `json:"message"`
	}
	_ = ctx.Request().Bind(&req)
	req.Message = strings.TrimSpace(req.Message)
	if req.Message == "" {
		req.Message = "维护已结束。"
	}
	if len(req.Message) > 5000 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "message 不能超过 5000 字"})
	}

	repo := repositories.NewIncidentRepository()
	incident, err := repo.GetByID(id)
	if err != nil || incident.SourceType != "maintenance" || incident.Status != "active" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "只能关闭进行中的维护事件"})
	}
	now := time.Now()
	if err := repo.AddEvent(&models.IncidentEvent{
		IncidentID: id,
		EventType:  "resolved",
		Status:     "resolved",
		Message:    req.Message,
		OccurredAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": err.Error()})
	}
	if err := repo.Update(id, map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   now,
		"last_event_at": now,
		"updated_at":    now,
	}); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": err.Error()})
	}
	updated, _ := repo.GetByID(id)
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{"status": true, "data": updated})
}

func parseIncidentRouteID(ctx http.Context) (uint, error) {
	raw := ctx.Request().Route("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	return uint(id), err
}

type publicIncidentEvent struct {
	ID         uint      `json:"id"`
	EventType  string    `json:"event_type"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

type publicIncident struct {
	ID          uint                  `json:"id"`
	SourceType  string                `json:"source_type"`
	Title       string                `json:"title"`
	Status      string                `json:"status"`
	Impact      string                `json:"impact"`
	StartedAt   time.Time             `json:"started_at"`
	ResolvedAt  *time.Time            `json:"resolved_at"`
	LastEventAt time.Time             `json:"last_event_at"`
	Events      []publicIncidentEvent `json:"events"`
}

func (c *IncidentController) GetPublic(ctx http.Context) http.Response {
	incidents, err := repositories.NewIncidentRepository().List(50)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	data := make([]publicIncident, 0, len(incidents))
	for _, incident := range incidents {
		if incident == nil {
			continue
		}
		item := publicIncident{
			ID:          incident.ID,
			SourceType:  incident.SourceType,
			Title:       incident.Title,
			Status:      incident.Status,
			Impact:      incident.Impact,
			StartedAt:   incident.StartedAt,
			ResolvedAt:  incident.ResolvedAt,
			LastEventAt: incident.LastEventAt,
			Events:      []publicIncidentEvent{},
		}
		for _, event := range incident.Events {
			if event == nil {
				continue
			}
			item.Events = append(item.Events, publicIncidentEvent{
				ID:         event.ID,
				EventType:  event.EventType,
				Status:     event.Status,
				Message:    event.Message,
				OccurredAt: event.OccurredAt,
			})
		}
		data = append(data, item)
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   data,
	})
}

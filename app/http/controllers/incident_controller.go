package controllers

import (
	"encoding/json"
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

	data := make([]map[string]interface{}, 0, len(incidents))
	for _, incident := range incidents {
		if incident == nil {
			continue
		}
		raw, err := json.Marshal(incident)
		if err != nil {
			continue
		}
		item := map[string]interface{}{}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		item["page_ids"] = parseIncidentPageIDs(incident.PageIDs)
		data = append(data, item)
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   data,
	})
}

func (c *IncidentController) CreateMaintenance(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	var req struct {
		Title   string   `json:"title"`
		Message string   `json:"message"`
		Impact  string   `json:"impact"`
		PageIDs []string `json:"page_ids"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "参数错误"})
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Message = strings.TrimSpace(req.Message)
	req.Impact = strings.TrimSpace(req.Impact)
	if req.Title == "" || req.Message == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "title 和 message 不能为空"})
	}
	if len(req.Title) > 255 || len(req.Message) > 5000 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "内容过长"})
	}
	if req.Impact == "" {
		req.Impact = "maintenance"
	}
	if req.Impact != "outage" && req.Impact != "degraded" && req.Impact != "maintenance" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "impact 不合法"})
	}

	pageIDs := make([]string, 0, len(req.PageIDs))
	seen := map[string]struct{}{}
	for _, id := range req.PageIDs {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 64 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		pageIDs = append(pageIDs, id)
	}
	if len(pageIDs) > 50 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "page_ids 过多（最大 50）"})
	}

	var pageIDsRaw *string
	if len(pageIDs) > 0 {
		encoded, err := json.Marshal(pageIDs)
		if err != nil {
			return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{"status": false, "message": "page_ids 编码失败"})
		}
		s := string(encoded)
		pageIDsRaw = &s
	}

	now := time.Now()
	repo := repositories.NewIncidentRepository()
	incident := &models.Incident{
		SourceType:  "maintenance",
		SourceID:    "manual",
		Title:       req.Title,
		Status:      "active",
		Impact:      req.Impact,
		PageIDs:     pageIDsRaw,
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
		Status:     req.Impact,
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
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "只能更新进行中的手动事件"})
	}
	now := time.Now()
	if err := repo.AddEvent(&models.IncidentEvent{
		IncidentID: id,
		EventType:  "update",
		Status:     incident.Impact,
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
		req.Message = "事件已结束。"
	}
	if len(req.Message) > 5000 {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "message 不能超过 5000 字"})
	}

	repo := repositories.NewIncidentRepository()
	incident, err := repo.GetByID(id)
	if err != nil || incident.SourceType != "maintenance" || incident.Status != "active" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "只能关闭进行中的手动事件"})
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
	SourceID    string                `json:"source_id"`
	Title       string                `json:"title"`
	Status      string                `json:"status"`
	Impact      string                `json:"impact"`
	PageIDs     []string              `json:"page_ids"`
	StartedAt   time.Time             `json:"started_at"`
	ResolvedAt  *time.Time            `json:"resolved_at"`
	LastEventAt time.Time             `json:"last_event_at"`
	Events      []publicIncidentEvent `json:"events"`
}

func parseIncidentPageIDs(raw *string) []string {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(*raw), &ids); err != nil {
		return []string{}
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func (c *IncidentController) GetPublic(ctx http.Context) http.Response {
	path := strings.TrimSpace(ctx.Request().Query("path", ""))
	if path == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "path 参数必填",
		})
	}

	resolution, err := getPublicPagePolicy().Resolve(path)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}
	filter := resolution.IncidentFilter
	if filter.Empty {
		return ctx.Response().Json(http.StatusOK, map[string]interface{}{
			"status": true,
			"data":   []publicIncident{},
		})
	}

	incidents, err := repositories.NewIncidentRepository().List(200)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	data := make([]publicIncident, 0, filter.Limit)
	for _, incident := range incidents {
		if !matchPublicIncident(incident, filter) {
			continue
		}
		item := publicIncident{
			ID:          incident.ID,
			SourceType:  incident.SourceType,
			SourceID:    incident.SourceID,
			Title:       incident.Title,
			Status:      incident.Status,
			Impact:      incident.Impact,
			PageIDs:     parseIncidentPageIDs(incident.PageIDs),
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
		if len(data) >= filter.Limit {
			break
		}
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   data,
	})
}

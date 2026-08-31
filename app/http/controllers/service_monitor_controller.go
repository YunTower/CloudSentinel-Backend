package controllers

import (
	"fmt"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
)

type ServiceMonitorController struct{}

func NewServiceMonitorController() *ServiceMonitorController {
	return &ServiceMonitorController{}
}

func (c *ServiceMonitorController) GetAll(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	repo := repositories.GetServiceMonitorRepository()
	monitors, err := repo.GetAll()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	// 附加最近的历史记录到每个监测器，用于状态条展示。
	if len(monitors) > 0 {
		ids := make([]uint, len(monitors))
		for i, m := range monitors {
			ids[i] = m.ID
		}
		histRepo := repositories.GetServiceMonitorHistoryRepository()
		histMap, _ := histRepo.GetBatchLast(ids, 60)
		resultRepo := repositories.NewServiceMonitorResultRepository()
		uptime24h, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-24*time.Hour))
		uptime7d, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-7*24*time.Hour))
		uptime30d, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-30*24*time.Hour))
		for _, m := range monitors {
			m.HasAIAPIKey = m.AIAPIKeyEncrypted != ""
			if entries, ok := histMap[m.ID]; ok {
				m.History = entries
			} else {
				m.History = []*models.ServiceMonitorHistory{}
			}
			m.Uptime = map[string]models.UptimeStat{
				"24h": uptime24h[m.ID],
				"7d":  uptime7d[m.ID],
				"30d": uptime30d[m.ID],
			}
		}
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true, "data": monitors,
	})
}

func (c *ServiceMonitorController) GetResults(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "无效ID",
		})
	}

	limit := 100
	if raw := ctx.Request().Input("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results, err := repositories.NewServiceMonitorResultRepository().GetLast(uint(id), limit)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   results,
	})
}

type publicServiceMonitor struct {
	ID                uint                            `json:"id"`
	Name              string                          `json:"name"`
	GroupName         string                          `json:"group_name"`
	Type              string                          `json:"type"`
	Status            string                          `json:"status"`
	ResponseTime      int                             `json:"response_time"`
	LastCheckAt       *time.Time                      `json:"last_check_at"`
	CheckCertExpiry   bool                            `json:"check_cert_expiry"`
	CertExpiresAt     *time.Time                      `json:"cert_expires_at"`
	CertDaysLeft      *int                            `json:"cert_days_left"`
	History           []*models.ServiceMonitorHistory `json:"history"`
	Uptime            map[string]models.UptimeStat    `json:"uptime,omitempty"`
	AIAPIFormat       string                          `json:"ai_api_format,omitempty"`
	AIModel           string                          `json:"ai_model,omitempty"`
	LastMetadata      map[string]any                  `json:"last_metadata,omitempty"`
	MetadataCheckedAt *time.Time                      `json:"metadata_checked_at,omitempty"`
}

func (c *ServiceMonitorController) GetPublic(ctx http.Context) http.Response {
	// 公开展示总开关关闭时与 /api/public/servers 行为一致：返回空列表，
	// 避免开关关闭后监测名称/状态/90 天 uptime/错误信息仍全量暴露
	if !loadPublicDisplayConfigV1().Enabled {
		return ctx.Response().Json(http.StatusOK, map[string]interface{}{
			"status": true,
			"data":   []publicServiceMonitor{},
		})
	}

	repo := repositories.GetServiceMonitorRepository()
	monitors, err := repo.GetAll()
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	enabled := make([]*models.ServiceMonitor, 0, len(monitors))
	for _, monitor := range monitors {
		if monitor != nil && monitor.Enabled {
			enabled = append(enabled, monitor)
		}
	}
	sort.SliceStable(enabled, func(i, j int) bool {
		if enabled[i].GroupName == enabled[j].GroupName {
			return enabled[i].Name < enabled[j].Name
		}
		return enabled[i].GroupName < enabled[j].GroupName
	})

	data := make([]publicServiceMonitor, 0, len(enabled))
	if len(enabled) > 0 {
		ids := make([]uint, len(enabled))
		for i, monitor := range enabled {
			ids[i] = monitor.ID
		}
		resultRepo := repositories.NewServiceMonitorResultRepository()
		dailyMap, _ := resultRepo.BatchDailyStatus(ids, 90)
		uptime24h, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-24*time.Hour))
		uptime7d, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-7*24*time.Hour))
		uptime30d, _ := resultRepo.BatchUptimeStats(ids, time.Now().Add(-30*24*time.Hour))
		var lastUpdated *time.Time
		for _, monitor := range enabled {
			daily := dailyMap[monitor.ID]
			history := make([]*models.ServiceMonitorHistory, 0, len(daily))
			for _, bar := range daily {
				history = append(history, &models.ServiceMonitorHistory{
					MonitorID:    monitor.ID,
					Status:       bar.Status,
					ResponseTime: bar.ResponseTime,
					CheckedAt:    bar.CheckedAt,
				})
			}
			if monitor.LastCheckAt != nil {
				if lastUpdated == nil || monitor.LastCheckAt.After(*lastUpdated) {
					t := *monitor.LastCheckAt
					lastUpdated = &t
				}
			}
			data = append(data, publicServiceMonitor{
				ID:                monitor.ID,
				Name:              monitor.Name,
				GroupName:         monitor.GroupName,
				Type:              monitor.Type,
				Status:            monitor.Status,
				ResponseTime:      monitor.ResponseTime,
				LastCheckAt:       monitor.LastCheckAt,
				CheckCertExpiry:   monitor.CheckCertExpiry,
				CertExpiresAt:     monitor.CertExpiresAt,
				CertDaysLeft:      monitor.CertDaysLeft,
				AIAPIFormat:       monitor.AIAPIFormat,
				AIModel:           monitor.AIModel,
				LastMetadata:      monitor.LastMetadata,
				MetadataCheckedAt: monitor.MetadataCheckedAt,
				History:           history,
				Uptime: map[string]models.UptimeStat{
					"24h": uptime24h[monitor.ID],
					"7d":  uptime7d[monitor.ID],
					"30d": uptime30d[monitor.ID],
				},
			})
		}

		return ctx.Response().Json(http.StatusOK, map[string]interface{}{
			"status": true,
			"data":   data,
			"meta": map[string]interface{}{
				"last_updated_at": lastUpdated,
			},
		})
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   data,
		"meta": map[string]interface{}{
			"last_updated_at": nil,
		},
	})
}

func (c *ServiceMonitorController) Create(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	var req struct {
		Name              string   `json:"name"`
		Type              string   `json:"type"`
		Target            string   `json:"target"`
		GroupName         string   `json:"group_name"`
		Port              int      `json:"port"`
		Interval          int      `json:"interval"`
		Timeout           int      `json:"timeout"`
		Enabled           bool     `json:"enabled"`
		ServerIDs         []string `json:"server_ids"`
		ExpectStatus      int      `json:"expect_status"`
		ExpectBody        string   `json:"expect_body"`
		HTTPMethod        string   `json:"http_method"`
		HTTPHeaders       string   `json:"http_headers"`
		HTTPBody          string   `json:"http_body"`
		FailureThreshold  int      `json:"failure_threshold"`
		RecoveryThreshold int      `json:"recovery_threshold"`
		CheckCertExpiry   bool     `json:"check_cert_expiry"`
		AIAPIFormat       string   `json:"ai_api_format"`
		AIModel           string   `json:"ai_model"`
		AIAPIKey          string   `json:"ai_api_key"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}
	if req.Name == "" || req.Type == "" || req.Target == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "name, type, target 不能为空",
		})
	}
	if req.Interval <= 0 {
		req.Interval = 60
	}
	if req.Timeout <= 0 {
		req.Timeout = 10
	}
	if req.FailureThreshold <= 0 {
		req.FailureThreshold = 1
	}
	if req.RecoveryThreshold <= 0 {
		req.RecoveryThreshold = 1
	}

	m := &models.ServiceMonitor{
		Name:              req.Name,
		Type:              req.Type,
		Target:            req.Target,
		GroupName:         strings.TrimSpace(req.GroupName),
		Port:              req.Port,
		Interval:          req.Interval,
		Timeout:           req.Timeout,
		Enabled:           req.Enabled,
		Status:            "unknown",
		ServerIDs:         req.ServerIDs,
		ExpectStatus:      req.ExpectStatus,
		ExpectBody:        req.ExpectBody,
		HTTPMethod:        normalizeHTTPMethod(req.HTTPMethod),
		HTTPHeaders:       req.HTTPHeaders,
		HTTPBody:          req.HTTPBody,
		FailureThreshold:  req.FailureThreshold,
		RecoveryThreshold: req.RecoveryThreshold,
		CheckCertExpiry:   req.Type == "https" && req.CheckCertExpiry,
		AIAPIFormat:       strings.TrimSpace(req.AIAPIFormat),
		AIModel:           strings.TrimSpace(req.AIModel),
	}
	if err := prepareProtocolMonitor(m, req.AIAPIKey); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	repo := repositories.GetServiceMonitorRepository()
	if err := repo.Create(m); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	if m.Enabled {
		services.GetServiceMonitorService().Start(m)
	}
	m.HasAIAPIKey = m.AIAPIKeyEncrypted != ""

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true, "data": m,
	})
}

func (c *ServiceMonitorController) Update(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "无效ID",
		})
	}

	var req struct {
		Name              string   `json:"name"`
		Type              string   `json:"type"`
		Target            string   `json:"target"`
		GroupName         *string  `json:"group_name"`
		Port              int      `json:"port"`
		Interval          int      `json:"interval"`
		Timeout           int      `json:"timeout"`
		Enabled           *bool    `json:"enabled"`
		ServerIDs         []string `json:"server_ids"`
		ExpectStatus      *int     `json:"expect_status"`
		ExpectBody        *string  `json:"expect_body"`
		HTTPMethod        *string  `json:"http_method"`
		HTTPHeaders       *string  `json:"http_headers"`
		HTTPBody          *string  `json:"http_body"`
		FailureThreshold  *int     `json:"failure_threshold"`
		RecoveryThreshold *int     `json:"recovery_threshold"`
		CheckCertExpiry   *bool    `json:"check_cert_expiry"`
		AIAPIFormat       *string  `json:"ai_api_format"`
		AIModel           *string  `json:"ai_model"`
		AIAPIKey          *string  `json:"ai_api_key"`
		ClearAIAPIKey     *bool    `json:"clear_ai_api_key"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}

	repo := repositories.GetServiceMonitorRepository()
	existing, existingErr := repo.GetByID(uint(id))
	if existingErr != nil || existing == nil {
		return ctx.Response().Json(http.StatusNotFound, map[string]interface{}{
			"status": false, "message": "监测任务不存在",
		})
	}
	monitorType := req.Type
	if monitorType == "" {
		monitorType = existing.Type
	}
	data := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if req.Name != "" {
		data["name"] = req.Name
	}
	if req.Type != "" {
		data["type"] = req.Type
		data["last_metadata"] = nil
		data["metadata_checked_at"] = nil
		if req.Type != "ai_model" {
			data["ai_api_format"] = ""
			data["ai_model"] = ""
			data["ai_api_key_encrypted"] = ""
		}
	}
	if req.Target != "" {
		target := req.Target
		if isPanelOnlyMonitorType(monitorType) {
			target = strings.TrimSpace(target)
		}
		data["target"] = target
	}
	if req.GroupName != nil {
		data["group_name"] = strings.TrimSpace(*req.GroupName)
	}
	if req.Port > 0 {
		data["port"] = req.Port
	}
	if req.Interval > 0 {
		data["interval"] = req.Interval
	}
	if req.Timeout > 0 {
		data["timeout"] = req.Timeout
	}
	if req.Enabled != nil {
		data["enabled"] = *req.Enabled
	}
	if req.ServerIDs != nil {
		if isPanelOnlyMonitorType(monitorType) && len(req.ServerIDs) > 0 {
			return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
				"status": false, "message": "Minecraft 与 AI 模型监测当前仅支持面板直接检测",
			})
		}
		data["server_ids"] = req.ServerIDs
	}
	if isPanelOnlyMonitorType(monitorType) {
		data["server_ids"] = []string{}
	}
	if req.ExpectStatus != nil {
		data["expect_status"] = *req.ExpectStatus
	}
	if req.ExpectBody != nil {
		data["expect_body"] = *req.ExpectBody
	}
	if req.HTTPMethod != nil {
		data["http_method"] = normalizeHTTPMethod(*req.HTTPMethod)
	}
	if req.HTTPHeaders != nil {
		data["http_headers"] = *req.HTTPHeaders
	}
	if req.HTTPBody != nil {
		data["http_body"] = *req.HTTPBody
	}
	if req.FailureThreshold != nil && *req.FailureThreshold > 0 {
		data["failure_threshold"] = *req.FailureThreshold
	}
	if req.RecoveryThreshold != nil && *req.RecoveryThreshold > 0 {
		data["recovery_threshold"] = *req.RecoveryThreshold
	}
	if err := validateProtocolMonitorUpdate(existing, req.Type, req.Target, req.AIAPIFormat, req.AIModel, req.AIAPIKey, req.ClearAIAPIKey); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": err.Error()})
	}
	if req.AIAPIFormat != nil {
		format := strings.TrimSpace(*req.AIAPIFormat)
		if monitorType == "ai_model" && !isSupportedAIAPIFormat(format) {
			return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "不支持的 AI 接口格式"})
		}
		data["ai_api_format"] = format
	}
	if req.AIModel != nil {
		model := strings.TrimSpace(*req.AIModel)
		if monitorType == "ai_model" && model == "" {
			return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": "AI 模型不能为空"})
		}
		data["ai_model"] = model
	}
	if req.ClearAIAPIKey != nil && *req.ClearAIAPIKey {
		data["ai_api_key_encrypted"] = ""
	} else if req.AIAPIKey != nil && strings.TrimSpace(*req.AIAPIKey) != "" {
		encrypted, encryptErr := encryptAIAPIKey(*req.AIAPIKey)
		if encryptErr != nil {
			return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{"status": false, "message": encryptErr.Error()})
		}
		data["ai_api_key_encrypted"] = encrypted
	}
	if req.CheckCertExpiry != nil {
		data["check_cert_expiry"] = monitorType == "https" && *req.CheckCertExpiry
		if monitorType != "https" || !*req.CheckCertExpiry {
			data["cert_expires_at"] = nil
			data["cert_days_left"] = nil
		}
	} else if monitorType != "" && monitorType != "https" {
		data["check_cert_expiry"] = false
		data["cert_expires_at"] = nil
		data["cert_days_left"] = nil
	}

	if err := repo.Update(uint(id), data); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	// 重新启动监测器并使用新的配置
	svc := services.GetServiceMonitorService()
	svc.Stop(uint(id))
	m, err := repo.GetByID(uint(id))
	if m != nil {
		m.HasAIAPIKey = m.AIAPIKeyEncrypted != ""
	}
	if err == nil && m.Enabled {
		svc.Start(m)
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true, "data": m,
	})
}

func (c *ServiceMonitorController) Delete(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	idStr := ctx.Request().Route("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "无效ID",
		})
	}

	services.GetServiceMonitorService().Stop(uint(id))

	repo := repositories.GetServiceMonitorRepository()
	if err := repo.Delete(uint(id)); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	// 关闭该监测项的未决事件并清理告警状态，避免孤儿数据
	services.NewIncidentService().ResolveForDeletedMonitor(uint(id))
	if err := repositories.NewAlertStateRepository().DeleteByMetricPrefix(fmt.Sprintf("service_monitor:%d:", uint(id))); err != nil {
		facades.Log().Warningf("清理监测项告警状态失败: monitor_id=%d, error=%v", uint(id), err)
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

func normalizeHTTPMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "GET"
	}
}

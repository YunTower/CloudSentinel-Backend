package controllers

import (
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"strconv"
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

	// 附加最近的24条历史记录到每个监测器
	if len(monitors) > 0 {
		ids := make([]uint, len(monitors))
		for i, m := range monitors {
			ids[i] = m.ID
		}
		histRepo := repositories.GetServiceMonitorHistoryRepository()
		histMap, _ := histRepo.GetBatchLast(ids, 24)
		for _, m := range monitors {
			if entries, ok := histMap[m.ID]; ok {
				m.History = entries
			} else {
				m.History = []*models.ServiceMonitorHistory{}
			}
		}
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true, "data": monitors,
	})
}

func (c *ServiceMonitorController) Create(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	var req struct {
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		Target       string   `json:"target"`
		Port         int      `json:"port"`
		Interval     int      `json:"interval"`
		Timeout      int      `json:"timeout"`
		Enabled      bool     `json:"enabled"`
		ServerIDs    []string `json:"server_ids"`
		ExpectStatus int      `json:"expect_status"`
		ExpectBody   string   `json:"expect_body"`
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

	m := &models.ServiceMonitor{
		Name:         req.Name,
		Type:         req.Type,
		Target:       req.Target,
		Port:         req.Port,
		Interval:     req.Interval,
		Timeout:      req.Timeout,
		Enabled:      req.Enabled,
		Status:       "unknown",
		ServerIDs:    req.ServerIDs,
		ExpectStatus: req.ExpectStatus,
		ExpectBody:   req.ExpectBody,
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
		Name         string   `json:"name"`
		Type         string   `json:"type"`
		Target       string   `json:"target"`
		Port         int      `json:"port"`
		Interval     int      `json:"interval"`
		Timeout      int      `json:"timeout"`
		Enabled      *bool    `json:"enabled"`
		ServerIDs    []string `json:"server_ids"`
		ExpectStatus *int     `json:"expect_status"`
		ExpectBody   *string  `json:"expect_body"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}

	repo := repositories.GetServiceMonitorRepository()
	data := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if req.Name != "" {
		data["name"] = req.Name
	}
	if req.Type != "" {
		data["type"] = req.Type
	}
	if req.Target != "" {
		data["target"] = req.Target
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
		data["server_ids"] = req.ServerIDs
	}
	if req.ExpectStatus != nil {
		data["expect_status"] = *req.ExpectStatus
	}
	if req.ExpectBody != nil {
		data["expect_body"] = *req.ExpectBody
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

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

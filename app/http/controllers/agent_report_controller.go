package controllers

import (
	"goravel/app/services"

	"github.com/goravel/framework/contracts/http"
)

type AgentReportController struct{}

func NewAgentReportController() *AgentReportController {
	return &AgentReportController{}
}

func (c *AgentReportController) Report(ctx http.Context) http.Response {
	var req struct {
		AgentKey string      `json:"agent_key"`
		Type     string      `json:"type"`
		Data     interface{} `json:"data"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}
	if req.AgentKey == "" || req.Type == "" || req.Data == nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "agent_key, type, data 不能为空",
		})
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return ctx.Response().Json(http.StatusUnauthorized, map[string]interface{}{
			"status": false, "message": "认证失败",
		})
	}
	data, err := decodeAgentReportPayload(req.Data)
	if err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	ingestor := services.NewAgentReportIngestor(
		services.GetAgentDataSaver(),
		services.GetServiceMonitorService().HandleAgentResult,
	)
	if err := ingestor.Ingest(serverID, req.Type, data); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

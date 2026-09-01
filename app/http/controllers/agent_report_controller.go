package controllers

import (
	"github.com/goravel/framework/contracts/http"
	"goravel/app/services"
	"goravel/app/utils"
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
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "参数错误")
	}
	if req.AgentKey == "" || req.Type == "" || req.Data == nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "agent_key, type, data 不能为空")
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusUnauthorized, "认证失败")
	}
	data, err := decodeAgentReportPayload(req.Data)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	ingestor := services.NewAgentReportIngestor(
		services.GetAgentDataSaver(),
		services.GetServiceMonitorService().HandleAgentResult,
	)
	if err := ingestor.Ingest(serverID, req.Type, data); err != nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

package controllers

import (
	"goravel/app/repositories"
	"goravel/app/services"

	"github.com/goravel/framework/contracts/http"
)

type AgentTaskController struct{}

func NewAgentTaskController() *AgentTaskController {
	return &AgentTaskController{}
}

func (c *AgentTaskController) Pull(ctx http.Context) http.Response {
	var req struct {
		AgentKey string `json:"agent_key"`
		Limit    int    `json:"limit"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}
	if req.AgentKey == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "agent_key 不能为空",
		})
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return ctx.Response().Json(http.StatusUnauthorized, map[string]interface{}{
			"status": false, "message": "认证失败",
		})
	}

	tasks, err := repositories.NewAgentTaskRepository().ClaimPending(serverID, req.Limit)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   tasks,
	})
}

func (c *AgentTaskController) Complete(ctx http.Context) http.Response {
	var req struct {
		AgentKey   string `json:"agent_key"`
		TaskID     string `json:"task_id"`
		LeaseToken string `json:"lease_token"`
		Status     string `json:"status"`
		Error      string `json:"error"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}
	if req.AgentKey == "" || req.TaskID == "" || req.LeaseToken == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "agent_key、task_id 和 lease_token 不能为空",
		})
	}
	if req.Status != "failed" {
		req.Status = "succeeded"
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return ctx.Response().Json(http.StatusUnauthorized, map[string]interface{}{
			"status": false, "message": "认证失败",
		})
	}

	completed, err := repositories.NewAgentTaskRepository().Complete(serverID, req.TaskID, req.LeaseToken, req.Status, req.Error)
	if err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}
	if !completed {
		return ctx.Response().Json(http.StatusConflict, map[string]interface{}{
			"status": false, "message": "任务租约无效、已过期或不属于当前 Agent",
		})
	}
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

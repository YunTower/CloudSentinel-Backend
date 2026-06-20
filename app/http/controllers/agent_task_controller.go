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

	tasks, err := repositories.NewAgentTaskRepository().PullPending(serverID, req.Limit)
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
		AgentKey string `json:"agent_key"`
		TaskID   string `json:"task_id"`
		Status   string `json:"status"`
		Error    string `json:"error"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "参数错误",
		})
	}
	if req.AgentKey == "" || req.TaskID == "" {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": "agent_key 和 task_id 不能为空",
		})
	}
	if req.Status != "failed" {
		req.Status = "succeeded"
	}

	if _, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip()); err != nil {
		return ctx.Response().Json(http.StatusUnauthorized, map[string]interface{}{
			"status": false, "message": "认证失败",
		})
	}

	if err := repositories.NewAgentTaskRepository().Complete(req.TaskID, req.Status, req.Error); err != nil {
		return ctx.Response().Json(http.StatusInternalServerError, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

package controllers

import (
	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"
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
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "参数错误")
	}
	if req.AgentKey == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "agent_key 不能为空")
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusUnauthorized, "认证失败")
	}

	tasks, err := repositories.NewAgentTaskRepository().ClaimPending(serverID, req.Limit)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	// HTTP 任务通道没有 WS 加密保护，任务载荷附带面板签名供 Agent 验签；
	// 签名失败的任务跳过（留在队列等待密钥恢复后重试），
	// 发送未签名任务对 Agent 端只会被拒绝
	taskPayloads := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		sig, ts, sigErr := services.SignAgentCommand(task.Payload, task.CommandID)
		if sigErr != nil {
			facades.Log().Errorf("Agent 任务签名失败，跳过下发: task_id=%s, error=%v", task.ID, sigErr)
			continue
		}
		payload := map[string]interface{}{
			"id":          task.ID,
			"command":     task.Command,
			"command_id":  task.CommandID,
			"lease_token": task.LeaseToken,
			"data":        task.Payload,
			"sig":         sig,
			"sig_ts":      ts,
		}
		taskPayloads = append(taskPayloads, payload)
	}
	return utils.SuccessDataResponse(ctx, taskPayloads)
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
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "参数错误")
	}
	if req.AgentKey == "" || req.TaskID == "" || req.LeaseToken == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "agent_key、task_id 和 lease_token 不能为空")
	}
	if req.Status != "failed" && req.Status != "cancelled" {
		req.Status = "succeeded"
	}

	serverID, err := services.GetAgentAuthValidator().ValidateAgentAuth(req.AgentKey, ctx.Request().Ip())
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusUnauthorized, "认证失败")
	}

	completed, err := repositories.NewAgentTaskRepository().Complete(serverID, req.TaskID, req.LeaseToken, req.Status, req.Error)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, err.Error())
	}
	if !completed {
		return utils.ErrorResponse(ctx, http.StatusConflict, "任务租约无效、已过期或不属于当前 Agent")
	}
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

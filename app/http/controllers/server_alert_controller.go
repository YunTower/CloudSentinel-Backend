package controllers

import (
	"encoding/json"
	"goravel/app/models"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
)

type ServerAlertController struct{}

func NewServerAlertController() *ServerAlertController {
	return &ServerAlertController{}
}

// GetServerAlertRules 获取指定服务器的告警规则
func (c *ServerAlertController) GetServerAlertRules(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "服务器ID不能为空")
	}

	alertService := services.NewAlertService()
	serverIDPtr := &serverID
	rules, err := alertService.GetServerRules(serverIDPtr)
	if err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "获取告警规则失败", err)
	}

	// 转换为前端需要的格式
	result := map[string]interface{}{
		"cpu": map[string]interface{}{
			"enabled":  rules.CPU.Enabled,
			"warning":  rules.CPU.Warning,
			"critical": rules.CPU.Critical,
		},
		"memory": map[string]interface{}{
			"enabled":  rules.Memory.Enabled,
			"warning":  rules.Memory.Warning,
			"critical": rules.Memory.Critical,
		},
		"disk": map[string]interface{}{
			"enabled":  rules.Disk.Enabled,
			"warning":  rules.Disk.Warning,
			"critical": rules.Disk.Critical,
		},
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data":    result,
	})
}

// UpdateServerAlertRules 更新指定服务器的告警规则
func (c *ServerAlertController) UpdateServerAlertRules(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "服务器ID不能为空")
	}

	type RuleInput struct {
		Enabled  bool    `json:"enabled" form:"enabled"`
		Warning  float64 `json:"warning" form:"warning"`
		Critical float64 `json:"critical" form:"critical"`
	}

	type RulesInput struct {
		CPU    *RuleInput `json:"cpu" form:"cpu"`
		Memory *RuleInput `json:"memory" form:"memory"`
		Disk   *RuleInput `json:"disk" form:"disk"`
		// 新增规则类型
		Bandwidth  *map[string]interface{} `json:"bandwidth" form:"bandwidth"`   // {enabled: bool, threshold: float64}
		Traffic    *map[string]interface{} `json:"traffic" form:"traffic"`       // {enabled: bool, threshold_percent: float64}
		Expiration *map[string]interface{} `json:"expiration" form:"expiration"` // {enabled: bool, alert_days: float64}
	}

	var req RulesInput
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, http.StatusBadRequest, "请求参数错误", err)
	}

	alertService := services.NewAlertService()
	serverIDPtr := &serverID
	rules := make(map[string]services.Rule)

	// 处理基础资源规则
	if req.CPU != nil {
		rules["cpu"] = services.Rule{
			Enabled:  req.CPU.Enabled,
			Warning:  req.CPU.Warning,
			Critical: req.CPU.Critical,
		}
	}
	if req.Memory != nil {
		rules["memory"] = services.Rule{
			Enabled:  req.Memory.Enabled,
			Warning:  req.Memory.Warning,
			Critical: req.Memory.Critical,
		}
	}
	if req.Disk != nil {
		rules["disk"] = services.Rule{
			Enabled:  req.Disk.Enabled,
			Warning:  req.Disk.Warning,
			Critical: req.Disk.Critical,
		}
	}

	// 处理新增规则类型（需要特殊处理，因为结构不同）
	ruleRepo := repositories.GetServerAlertRuleRepository()
	if req.Bandwidth != nil {
		configJson, _ := json.Marshal(*req.Bandwidth)
		rule := &models.ServerAlertRule{
			ServerID: serverIDPtr,
			RuleType: "bandwidth",
			Config:   string(configJson),
		}
		_ = ruleRepo.CreateOrUpdate(rule)
	}
	if req.Traffic != nil {
		configJson, _ := json.Marshal(*req.Traffic)
		rule := &models.ServerAlertRule{
			ServerID: serverIDPtr,
			RuleType: "traffic",
			Config:   string(configJson),
		}
		_ = ruleRepo.CreateOrUpdate(rule)
	}
	if req.Expiration != nil {
		configJson, _ := json.Marshal(*req.Expiration)
		rule := &models.ServerAlertRule{
			ServerID: serverIDPtr,
			RuleType: "expiration",
			Config:   string(configJson),
		}
		_ = ruleRepo.CreateOrUpdate(rule)
	}

	// 保存基础资源规则
	if len(rules) > 0 {
		if err := alertService.SaveServerRules(serverIDPtr, rules); err != nil {
			return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "保存告警规则失败", err)
		}
	}

	return utils.SuccessResponse(ctx, "success")
}


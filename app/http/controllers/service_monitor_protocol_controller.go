package controllers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/monitorprobe"
	"goravel/app/services"
	"goravel/app/utils"
	"goravel/app/utils/secret"
)

func isPanelOnlyMonitorType(monitorType string) bool {
	switch monitorType {
	case monitorprobe.TypeMinecraftJava, monitorprobe.TypeMinecraftBedrock, monitorprobe.TypeAIModel:
		return true
	default:
		return false
	}
}

func isSupportedAIAPIFormat(format string) bool {
	switch format {
	case monitorprobe.AIFormatAnthropicMessages, monitorprobe.AIFormatChatCompletions, monitorprobe.AIFormatResponses:
		return true
	default:
		return false
	}
}

func validateFullHTTPURL(value string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("AI 接口地址必须是完整的 HTTP 或 HTTPS URL")
	}
	return nil
}

func encryptAIAPIKey(value string) (string, error) {
	if !secret.HasAppKey() {
		return "", fmt.Errorf("当前实例未配置 APP_KEY，无法安全保存 AI API Key")
	}
	return secret.EncryptStringWithAppKey(strings.TrimSpace(value))
}

func prepareProtocolMonitor(monitor *models.ServiceMonitor, apiKey string) error {
	if !isPanelOnlyMonitorType(monitor.Type) {
		return nil
	}
	if len(monitor.ServerIDs) > 0 {
		return fmt.Errorf("Minecraft 与 AI 模型监测当前仅支持面板直接检测")
	}
	monitor.ServerIDs = []string{}
	monitor.CheckCertExpiry = false
	monitor.Target = strings.TrimSpace(monitor.Target)
	switch monitor.Type {
	case monitorprobe.TypeMinecraftJava:
		if monitor.Port <= 0 {
			monitor.Port = 25565
		}
	case monitorprobe.TypeMinecraftBedrock:
		if monitor.Port <= 0 {
			monitor.Port = 19132
		}
	case monitorprobe.TypeAIModel:
		if err := validateFullHTTPURL(monitor.Target); err != nil {
			return err
		}
		if !isSupportedAIAPIFormat(monitor.AIAPIFormat) {
			return fmt.Errorf("不支持的 AI 接口格式")
		}
		if monitor.AIModel == "" {
			return fmt.Errorf("AI 模型不能为空")
		}
		if strings.TrimSpace(apiKey) == "" {
			return fmt.Errorf("AI API Key 不能为空")
		}
		encrypted, err := encryptAIAPIKey(apiKey)
		if err != nil {
			return err
		}
		monitor.AIAPIKeyEncrypted = encrypted
	}
	return nil
}

func validateProtocolMonitorUpdate(existing *models.ServiceMonitor, requestType, requestTarget string, requestFormat, requestModel, requestKey *string, clearKey *bool) error {
	monitorType := requestType
	if monitorType == "" {
		monitorType = existing.Type
	}
	if !isPanelOnlyMonitorType(monitorType) {
		return nil
	}
	target := existing.Target
	if strings.TrimSpace(requestTarget) != "" {
		target = strings.TrimSpace(requestTarget)
	}
	if monitorType != monitorprobe.TypeAIModel {
		if target == "" {
			return fmt.Errorf("Minecraft 服务器地址不能为空")
		}
		return nil
	}
	if err := validateFullHTTPURL(target); err != nil {
		return err
	}
	format := existing.AIAPIFormat
	if requestFormat != nil {
		format = strings.TrimSpace(*requestFormat)
	}
	if !isSupportedAIAPIFormat(format) {
		return fmt.Errorf("不支持的 AI 接口格式")
	}
	model := existing.AIModel
	if requestModel != nil {
		model = strings.TrimSpace(*requestModel)
	}
	if model == "" {
		return fmt.Errorf("AI 模型不能为空")
	}
	hasKey := existing.AIAPIKeyEncrypted != ""
	if requestKey != nil && strings.TrimSpace(*requestKey) != "" {
		hasKey = true
	}
	if clearKey != nil && *clearKey {
		hasKey = false
	}
	if !hasKey {
		return fmt.Errorf("AI API Key 不能为空")
	}
	return nil
}

// CreateAIModels 将模型列表展开为一模型一监测项，并在同一事务中创建。
func (c *ServiceMonitorController) CreateAIModels(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	var req struct {
		NamePrefix        string   `json:"name_prefix"`
		Target            string   `json:"target"`
		GroupName         string   `json:"group_name"`
		AIAPIFormat       string   `json:"ai_api_format"`
		AIModels          []string `json:"ai_models"`
		AIAPIKey          string   `json:"ai_api_key"`
		Interval          int      `json:"interval"`
		Timeout           int      `json:"timeout"`
		Enabled           bool     `json:"enabled"`
		FailureThreshold  int      `json:"failure_threshold"`
		RecoveryThreshold int      `json:"recovery_threshold"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "参数错误")
	}
	modelsList := make([]string, 0, len(req.AIModels))
	seen := make(map[string]struct{}, len(req.AIModels))
	for _, value := range req.AIModels {
		model := strings.TrimSpace(value)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		modelsList = append(modelsList, model)
	}
	if len(modelsList) == 0 || len(modelsList) > 50 {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "模型数量必须在 1 到 50 之间")
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
	prefix := strings.TrimSpace(req.NamePrefix)
	created := make([]*models.ServiceMonitor, 0, len(modelsList))
	for _, modelName := range modelsList {
		name := modelName
		if prefix != "" {
			name = prefix + " · " + modelName
		}
		monitor := &models.ServiceMonitor{
			Name: name, Type: monitorprobe.TypeAIModel, Target: strings.TrimSpace(req.Target),
			GroupName: strings.TrimSpace(req.GroupName), Interval: req.Interval, Timeout: req.Timeout,
			Enabled: req.Enabled, Status: "unknown", ServerIDs: []string{},
			FailureThreshold: req.FailureThreshold, RecoveryThreshold: req.RecoveryThreshold,
			AIAPIFormat: strings.TrimSpace(req.AIAPIFormat), AIModel: modelName,
		}
		if err := prepareProtocolMonitor(monitor, req.AIAPIKey); err != nil {
			return utils.ErrorResponse(ctx, http.StatusBadRequest, err.Error())
		}
		created = append(created, monitor)
	}
	if err := facades.Orm().Transaction(func(tx orm.Query) error {
		for _, monitor := range created {
			if err := tx.Create(monitor); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, err.Error())
	}
	for _, monitor := range created {
		monitor.HasAIAPIKey = true
		if monitor.Enabled {
			services.GetServiceMonitorService().Start(monitor)
		}
	}
	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true, "data": created, "created_count": len(created),
	})
}

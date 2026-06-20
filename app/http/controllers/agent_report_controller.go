package controllers

import (
	"errors"
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

	saver := services.GetAgentDataSaver()
	if err := saveAgentReportPayload(saver, serverID, req.Type, data); err != nil {
		return ctx.Response().Json(http.StatusBadRequest, map[string]interface{}{
			"status": false, "message": err.Error(),
		})
	}

	return ctx.Response().Json(http.StatusOK, map[string]interface{}{
		"status": true,
	})
}

func saveAgentReportPayload(saver interface {
	SaveSystemInfo(string, map[string]interface{}) error
	SaveMetrics(string, map[string]interface{}) error
	SaveMemoryInfo(string, map[string]interface{}) error
	SaveDiskInfo(string, []interface{}) error
	SaveDiskIO(string, map[string]interface{}) error
	SaveNetworkInfo(string, map[string]interface{}) error
	SaveSwapInfo(string, map[string]interface{}) error
	SaveProcessInfo(string, map[string]interface{}) error
	SaveGPUInfo(string, map[string]interface{}) error
	SaveAgentLogs(string, []interface{}) error
}, serverID, reportType string, data interface{}) error {
	switch reportType {
	case "service_check_result":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		services.GetServiceMonitorService().HandleAgentResult(payload, serverID)
		return nil
	case "system_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveSystemInfo(serverID, payload)
	case "metrics":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveMetrics(serverID, payload)
	case "memory_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveMemoryInfo(serverID, payload)
	case "disk_info":
		payload, err := agentReportSlice(data)
		if err != nil {
			return err
		}
		return saver.SaveDiskInfo(serverID, payload)
	case "disk_io":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveDiskIO(serverID, payload)
	case "network_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveNetworkInfo(serverID, payload)
	case "swap_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveSwapInfo(serverID, payload)
	case "process_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveProcessInfo(serverID, payload)
	case "gpu_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return saver.SaveGPUInfo(serverID, payload)
	case "agent_log":
		payload, err := agentReportSlice(data)
		if err != nil {
			return err
		}
		return saver.SaveAgentLogs(serverID, payload)
	default:
		return &unsupportedAgentReportType{typ: reportType}
	}
}

func agentReportMap(data interface{}) (map[string]interface{}, error) {
	payload, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.New("data 必须是对象")
	}
	return payload, nil
}

func agentReportSlice(data interface{}) ([]interface{}, error) {
	payload, ok := data.([]interface{})
	if !ok {
		return nil, errors.New("data 必须是数组")
	}
	return payload, nil
}

type unsupportedAgentReportType struct {
	typ string
}

func (e *unsupportedAgentReportType) Error() string {
	return "不支持的上报类型: " + e.typ
}

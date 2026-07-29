package services

import (
	"errors"
	"fmt"

	"goravel/app/services/websocket"
)

// AgentReportIngestor is the single module that accepts a decoded Agent report
// envelope and routes it to the appropriate domain write. The HTTP fallback
// transport delegates its report-type policy here instead of owning a type switch.
type AgentReportIngestor struct {
	saver             websocket.AgentDataSaver
	handleCheckResult func(map[string]interface{}, string)
}

func NewAgentReportIngestor(saver websocket.AgentDataSaver, handleCheckResult func(map[string]interface{}, string)) *AgentReportIngestor {
	return &AgentReportIngestor{
		saver:             saver,
		handleCheckResult: handleCheckResult,
	}
}

// Ingest validates the report shape and queues or persists its domain data.
// A caller only needs to know the server identity, report type, and decoded
// payload; all report-specific routing stays inside this module.
func (i *AgentReportIngestor) Ingest(serverID, reportType string, data interface{}) error {
	switch reportType {
	case "service_check_result":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		if i.handleCheckResult == nil {
			return errors.New("服务监测结果处理器不可用")
		}
		i.handleCheckResult(payload, serverID)
		return nil
	case "system_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveSystemInfo(serverID, payload)
	case "metrics":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveMetrics(serverID, payload)
	case "memory_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveMemoryInfo(serverID, payload)
	case "disk_info":
		payload, err := agentReportSlice(data)
		if err != nil {
			return err
		}
		return i.saver.SaveDiskInfo(serverID, payload)
	case "disk_io":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveDiskIO(serverID, payload)
	case "network_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveNetworkInfo(serverID, payload)
	case "swap_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveSwapInfo(serverID, payload)
	case "process_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveProcessInfo(serverID, payload)
	case "gpu_info":
		payload, err := agentReportMap(data)
		if err != nil {
			return err
		}
		return i.saver.SaveGPUInfo(serverID, payload)
	case "agent_log":
		payload, err := agentReportSlice(data)
		if err != nil {
			return err
		}
		return i.saver.SaveAgentLogs(serverID, payload)
	default:
		return fmt.Errorf("不支持的上报类型: %s", reportType)
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

package services

import (
	"goravel/app/models"
	"goravel/app/services/websocket"

	"github.com/goravel/framework/facades"
)

// agentAuthValidator 实现 websocket.AgentAuthValidator 接口
type agentAuthValidator struct{}

// NewAgentAuthValidator 创建新的 AgentAuthValidator
func NewAgentAuthValidator() websocket.AgentAuthValidator {
	return &agentAuthValidator{}
}

// GetAgentAuthValidator 获取 AgentAuthValidator 实例
func GetAgentAuthValidator() websocket.AgentAuthValidator {
	return NewAgentAuthValidator()
}

func (v *agentAuthValidator) ValidateAgentAuth(agentKey string, clientIP string) (string, error) {
	var server models.Server
	if err := facades.Orm().Query().Where("agent_key = ?", agentKey).First(&server); err != nil {
		return "", err
	}
	// 可选：更新 IP
	if server.IP != clientIP {
		server.IP = clientIP
		facades.Orm().Query().Save(&server)
	}
	return server.ID, nil
}

// agentDataSaver 实现 websocket.AgentDataSaver 接口
type agentDataSaver struct{}

// NewAgentDataSaver 创建新的 AgentDataSaver
func NewAgentDataSaver() websocket.AgentDataSaver {
	return &agentDataSaver{}
}

// GetAgentDataSaver 获取 AgentDataSaver 实例
func GetAgentDataSaver() websocket.AgentDataSaver {
	return NewAgentDataSaver()
}

func (s *agentDataSaver) SaveSystemInfo(serverID string, data map[string]interface{}) error {
	return SaveSystemInfo(serverID, data)
}

func (s *agentDataSaver) SaveMetrics(serverID string, data map[string]interface{}) error {
	return SaveMetrics(serverID, data)
}

func (s *agentDataSaver) SaveMemoryInfo(serverID string, data map[string]interface{}) error {
	return SaveMemoryInfo(serverID, data)
}

func (s *agentDataSaver) SaveDiskInfo(serverID string, data []interface{}) error {
	return SaveDiskInfo(serverID, data)
}

func (s *agentDataSaver) SaveDiskIO(serverID string, data map[string]interface{}) error {
	return SaveDiskIO(serverID, data)
}

func (s *agentDataSaver) SaveNetworkInfo(serverID string, data map[string]interface{}) error {
	return SaveNetworkInfo(serverID, data)
}

func (s *agentDataSaver) SaveSwapInfo(serverID string, data map[string]interface{}) error {
	return SaveSwapInfo(serverID, data)
}

func (s *agentDataSaver) SaveProcessInfo(serverID string, data map[string]interface{}) error {
	return SaveProcessInfo(serverID, data)
}

func (s *agentDataSaver) SaveGPUInfo(serverID string, data map[string]interface{}) error {
	return SaveGPUInfo(serverID, data)
}

func (s *agentDataSaver) SaveAgentLogs(serverID string, logs []interface{}) error {
	return SaveAgentLogs(serverID, logs)
}

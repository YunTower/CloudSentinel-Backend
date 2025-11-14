package services

import (
	ws "goravel/app/services/websocket"
)

// agentAuthValidator 实现 AgentAuthValidator 接口
type agentAuthValidator struct{}

func (v *agentAuthValidator) ValidateAgentAuth(agentKey string, clientIP string) (string, error) {
	return ValidateAgentAuth(agentKey, clientIP)
}

// agentDataSaver 实现 AgentDataSaver 接口
type agentDataSaver struct{}

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

// GetAgentAuthValidator 获取 Agent 认证验证器
func GetAgentAuthValidator() ws.AgentAuthValidator {
	return &agentAuthValidator{}
}

// GetAgentDataSaver 获取 Agent 数据保存器
func GetAgentDataSaver() ws.AgentDataSaver {
	return &agentDataSaver{}
}


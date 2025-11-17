package websocket

// AgentAuthValidator Agent 认证验证器接口
type AgentAuthValidator interface {
	ValidateAgentAuth(agentKey string, clientIP string) (string, error)
}

// AgentDataSaver Agent 数据保存器接口
type AgentDataSaver interface {
	SaveSystemInfo(serverID string, data map[string]interface{}) error
	SaveMetrics(serverID string, data map[string]interface{}) error
	SaveMemoryInfo(serverID string, data map[string]interface{}) error
	SaveDiskInfo(serverID string, data []interface{}) error
	SaveDiskIO(serverID string, data map[string]interface{}) error
	SaveNetworkInfo(serverID string, data map[string]interface{}) error
	SaveSwapInfo(serverID string, data map[string]interface{}) error
}


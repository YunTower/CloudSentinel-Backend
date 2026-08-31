package services

import (
	"crypto/sha256"
	"encoding/hex"

	"goravel/app/models"
	"goravel/app/services/websocket"

	"goravel/app/facades"
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

// HashAgentKey 计算 agent_key 的 SHA-256（认证查询走哈希索引）。
func HashAgentKey(agentKey string) string {
	sum := sha256.Sum256([]byte(agentKey))
	return hex.EncodeToString(sum[:])
}

func (v *agentAuthValidator) ValidateAgentAuth(agentKey string, clientIP string) (string, error) {
	var server models.Server
	// 优先按哈希查询；存量数据哈希为空时回退明文匹配并惰性回填。
	err := facades.Orm().Query().Where("agent_key_hash = ?", HashAgentKey(agentKey)).First(&server)
	if err != nil {
		if legacyErr := facades.Orm().Query().Where("agent_key = ?", agentKey).First(&server); legacyErr != nil {
			return "", legacyErr
		}
		facades.Orm().Query().Model(&models.Server{}).Where("id", server.ID).
			Update("agent_key_hash", HashAgentKey(server.AgentKey))
	}
	// 不再用连接 IP 无条件覆盖资产 IP：反代环境下该值可被 XFF 伪造，
	// 会污染服务器记录。仅当记录为空时写入一次。
	if server.IP == "" && clientIP != "" {
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

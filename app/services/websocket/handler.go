package websocket

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"goravel/app/cryptoutil"
	"goravel/app/repositories"

	"github.com/goravel/framework/facades"
)

// AgentMessageHandler Agent 消息处理器接口
type AgentMessageHandler interface {
	// HandleAuth 处理认证消息
	HandleAuth(data map[string]interface{}, conn *AgentConnection) error
	// HandleHeartbeat 处理心跳消息
	HandleHeartbeat(conn *AgentConnection) error
	// HandleSystemInfo 处理系统信息消息
	HandleSystemInfo(data map[string]interface{}, conn *AgentConnection) error
	// HandleMetrics 处理性能指标消息
	HandleMetrics(data map[string]interface{}, conn *AgentConnection) error
	// HandleMemoryInfo 处理内存信息消息
	HandleMemoryInfo(data map[string]interface{}, conn *AgentConnection) error
	// HandleDiskInfo 处理磁盘信息消息
	HandleDiskInfo(data map[string]interface{}, conn *AgentConnection) error
	// HandleDiskIO 处理磁盘IO消息
	HandleDiskIO(data map[string]interface{}, conn *AgentConnection) error
	// HandleNetworkInfo 处理网络信息消息
	HandleNetworkInfo(data map[string]interface{}, conn *AgentConnection) error
	// HandleSwapInfo 处理Swap内存消息
	HandleSwapInfo(data map[string]interface{}, conn *AgentConnection) error
	// HandleAgentConfig 处理Agent配置消息
	HandleAgentConfig(data map[string]interface{}, conn *AgentConnection) error
}

// FrontendMessageHandler Frontend 消息处理器接口
type FrontendMessageHandler interface {
	// HandlePing 处理心跳消息
	HandlePing(conn *FrontendConnection) error
}

// agentMessageHandler Agent 消息处理器实现
type agentMessageHandler struct {
	manager   ConnectionManager
	validator AgentAuthValidator
	saver     AgentDataSaver
}

// NewAgentMessageHandler 创建 Agent 消息处理器
func NewAgentMessageHandler(manager ConnectionManager, validator AgentAuthValidator, saver AgentDataSaver) AgentMessageHandler {
	return &agentMessageHandler{
		manager:   manager,
		validator: validator,
		saver:     saver,
	}
}

// HandleAuth 处理认证消息
func (h *agentMessageHandler) HandleAuth(data map[string]interface{}, conn *AgentConnection) error {
	authData, ok := data["data"].(map[string]interface{})
	if !ok {
		facades.Log().Channel("websocket").Warning("认证数据格式错误")
		return errors.New("认证数据格式错误")
	}

	agentKey, ok := authData["key"].(string)
	if !ok || agentKey == "" {
		facades.Log().Channel("websocket").Warningf("认证失败: 缺少agent key (IP: %s)", conn.GetRemoteAddr())
		return errors.New("缺少agent key")
	}

	// 验证 key 长度（UUID 格式应该是 36 个字符）
	if len(agentKey) != 36 {
		keyPreview := agentKey
		if len(keyPreview) > 8 {
			keyPreview = keyPreview[:8] + "..."
		}
		facades.Log().Channel("websocket").Warningf("警告: 接收到的 agent key 长度异常 (%d)，正常应该是 36 个字符，key: %s", len(agentKey), keyPreview)
	}

	authType, _ := authData["type"].(string)
	if authType != "server" {
		keyPreview := agentKey
		if len(keyPreview) > 8 {
			keyPreview = keyPreview[:8] + "..."
		}
		facades.Log().Channel("websocket").Warningf("认证失败: 不支持的认证类型 %s (IP: %s, key: %s)", authType, conn.GetRemoteAddr(), keyPreview)
		return errors.New("不支持的认证类型")
	}

	// 接收 Agent 公钥（可选，如果支持加密）
	agentPublicKey, _ := authData["agent_public_key"].(string)

	// 验证agent key和IP并获取server_id
	clientIP := conn.GetRemoteAddr()
	keyPreview := agentKey
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8] + "..."
	}
	facades.Log().Channel("websocket").Infof("尝试认证: IP=%s, key=%s (完整长度: %d)", clientIP, keyPreview, len(agentKey))

	serverID, err := h.validator.ValidateAgentAuth(agentKey, clientIP)
	if err != nil {
		facades.Log().Channel("websocket").Warningf("认证失败: %v (IP: %s, key: %s)", err, clientIP, keyPreview)
		return errors.New("认证失败: " + err.Error())
	}

	// 认证成功后，如果当前IP是127.0.0.1，使用服务器记录中的IP作为真实IP
	if strings.HasPrefix(clientIP, "127.0.0.1") || strings.HasPrefix(clientIP, "::1") || strings.HasPrefix(clientIP, "localhost") {
		var serverRecord []map[string]interface{}
		if err := facades.Orm().Query().Table("servers").
			Select("ip").
			Where("id", serverID).
			Get(&serverRecord); err == nil && len(serverRecord) > 0 {
			if serverIP, ok := serverRecord[0]["ip"].(string); ok && serverIP != "" && serverIP != "127.0.0.1" {
				conn.SetRemoteAddr(serverIP)
				facades.Log().Channel("websocket").Infof("检测到本地连接，使用服务器记录中的IP: %s (原连接IP: %s)", serverIP, clientIP)
			}
		}
	}

	// 处理密钥交换（如果 Agent 提供了公钥）
	// 注意：必须在设置连接状态前进行，因为指纹不匹配时需要拒绝连接
	if agentPublicKey != "" {
		if err := h.handleKeyExchange(serverID, agentPublicKey, conn); err != nil {
			facades.Log().Channel("websocket").Errorf("密钥交换失败: %v", err)
			// 密钥交换失败（特别是指纹不匹配）时拒绝连接，防止中间人攻击
			return fmt.Errorf("密钥交换失败，连接已拒绝: %w", err)
		}
	}

	// 更新连接信息（密钥交换成功后才设置）
	conn.SetServerID(serverID)
	conn.SetAgentKey(agentKey)
	conn.SetState(StateAuthenticated)
	conn.UpdateLastPing()

	// 先发送认证成功响应（在注册前发送，避免旧连接被关闭）
	response := map[string]interface{}{
		"type":    MessageTypeAuth,
		"status":  "success",
		"message": "认证成功",
		"data": map[string]interface{}{
			"server_id": serverID,
		},
	}

	if err := conn.WriteJSON(response); err != nil {
		facades.Log().Channel("websocket").Errorf("发送认证响应失败: %v", err)
		return err
	}

	facades.Log().Channel("websocket").Infof("Agent认证成功: server_id=%s, remote=%s", serverID, conn.GetRemoteAddr())

	// 注册连接（这会关闭旧连接）
	// 注意：必须在发送响应后注册，否则旧连接可能在响应发送前被关闭
	if err := h.manager.RegisterAgent(serverID, conn); err != nil {
		return err
	}

	return nil
}

// handleKeyExchange 处理密钥交换流程
func (h *agentMessageHandler) handleKeyExchange(serverID, agentPublicKey string, conn *AgentConnection) error {
	serverRepo := repositories.NewServerRepository()

	// 计算 Agent 公钥指纹
	agentFingerprint, err := h.getPublicKeyFingerprint(agentPublicKey)
	if err != nil {
		return err
	}

	// 从数据库查询服务器信息
	var serverKeys []map[string]interface{}
	err = facades.Orm().Query().Table("servers").
		Select("agent_fingerprint").
		Where("id", serverID).
		Get(&serverKeys)

	if err != nil || len(serverKeys) == 0 {
		return fmt.Errorf("获取服务器信息失败: %w", err)
	}

	serverKeyData := serverKeys[0]

	// 检查数据库中是否已有 Agent 公钥指纹
	if existingFingerprint, ok := serverKeyData["agent_fingerprint"].(string); ok && existingFingerprint != "" {
		if existingFingerprint != agentFingerprint {
			facades.Log().Channel("websocket").Errorf("Agent 公钥指纹不匹配: 期望=%s, 实际=%s", existingFingerprint, agentFingerprint)
			return errors.New("Agent 公钥指纹验证失败，可能存在中间人攻击")
		}
	}

	// 保存 Agent 公钥和指纹
	conn.SetAgentPublicKey(agentPublicKey)
	conn.SetAgentFingerprint(agentFingerprint)

	// 从 system_settings 获取或生成 panel 密钥对
	_, panelPublicKey, err := h.getOrGeneratePanelKeyPair()
	if err != nil {
		return fmt.Errorf("获取面板密钥对失败: %w", err)
	}

	// 计算面板公钥指纹
	panelFingerprint, err := h.getPublicKeyFingerprint(panelPublicKey)
	if err != nil {
		return err
	}

	// 发送面板公钥和指纹（明文）
	keyExchangeResponse := map[string]interface{}{
		"type":    "key_exchange",
		"status":  "success",
		"message": "密钥交换",
		"data": map[string]interface{}{
			"panel_public_key":  panelPublicKey,
			"panel_fingerprint": panelFingerprint,
		},
	}

	// 密钥交换消息使用明文发送（此时还未启用加密）
	if err := conn.WriteJSON(keyExchangeResponse); err != nil {
		return err
	}

	// 生成 AES 会话密钥
	sessionKey, err := h.generateSessionKey()
	if err != nil {
		return err
	}

	// 使用 Agent 公钥加密会话密钥
	encryptedSessionKey, err := h.encryptWithPublicKey(sessionKey, agentPublicKey)
	if err != nil {
		return err
	}

	// Base64 编码加密后的会话密钥
	encryptedSessionKeyBase64 := base64.StdEncoding.EncodeToString(encryptedSessionKey)

	// 发送加密的会话密钥（明文传输，但内容是加密的）
	sessionKeyResponse := map[string]interface{}{
		"type":    "session_key",
		"status":  "success",
		"message": "会话密钥",
		"data": map[string]interface{}{
			"encrypted_session_key": encryptedSessionKeyBase64,
		},
	}

	// 会话密钥消息使用明文发送（内容是加密的，但传输是明文的）
	if err := conn.WriteJSON(sessionKeyResponse); err != nil {
		return err
	}

	// 设置会话密钥并启用加密
	conn.SetSessionKey(sessionKey)
	conn.EnableEncryption()

	// 更新数据库中的 Agent 公钥和指纹
	updateData := map[string]interface{}{
		"agent_public_key":  agentPublicKey,
		"agent_fingerprint": agentFingerprint,
	}
	if err := serverRepo.Update(serverID, updateData); err != nil {
		facades.Log().Channel("websocket").Warningf("更新 Agent 公钥和指纹失败: %v", err)
		// 不影响加密启用
	}

	facades.Log().Channel("websocket").Infof("密钥交换成功: server_id=%s, encryption_enabled=true", serverID)

	return nil
}

// getOrGeneratePanelKeyPair 从 system_settings 获取或生成 panel 密钥对
func (h *agentMessageHandler) getOrGeneratePanelKeyPair() (privateKey, publicKey string, err error) {
	settingRepo := repositories.GetSystemSettingRepository()

	// 尝试从 system_settings 读取 panel 密钥对
	var panelKeys map[string]interface{}
	err = settingRepo.GetJSON("panel_rsa_keys", &panelKeys)

	if err == nil && panelKeys != nil {
		if pk, ok := panelKeys["panel_private_key"].(string); ok && pk != "" {
			if pub, ok := panelKeys["panel_public_key"].(string); ok && pub != "" {
				// 返回已有的密钥对
				return pk, pub, nil
			}
		}
	}

	// 如果不存在或无效，生成新的密钥对
	facades.Log().Channel("websocket").Info("Panel 密钥对不存在，正在生成新的密钥对...")

	privateKey, publicKey, err = cryptoutil.GenerateKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("生成密钥对失败: %w", err)
	}

	// 保存到 system_settings
	panelKeys = map[string]interface{}{
		"panel_private_key": privateKey,
		"panel_public_key":  publicKey,
	}

	if err := settingRepo.SetJSON("panel_rsa_keys", panelKeys); err != nil {
		facades.Log().Channel("websocket").Errorf("保存 Panel 密钥对到 system_settings 失败: %v", err)
		return "", "", fmt.Errorf("保存 Panel 密钥对失败: %w", err)
	}

	facades.Log().Channel("websocket").Info("Panel 密钥对已生成并保存到 system_settings")
	return privateKey, publicKey, nil
}

// getPublicKeyFingerprint 计算公钥指纹（SHA256）
func (h *agentMessageHandler) getPublicKeyFingerprint(publicKey string) (string, error) {
	return cryptoutil.GetPublicKeyFingerprint(publicKey)
}

// encryptWithPublicKey 使用公钥加密数据
func (h *agentMessageHandler) encryptWithPublicKey(data []byte, publicKey string) ([]byte, error) {
	return cryptoutil.EncryptWithPublicKey(data, publicKey)
}

// generateSessionKey 生成 AES-256 会话密钥（32字节）
func (h *agentMessageHandler) generateSessionKey() ([]byte, error) {
	return cryptoutil.GenerateSessionKey()
}

// HandleHeartbeat 处理心跳消息
func (h *agentMessageHandler) HandleHeartbeat(conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	h.manager.UpdateAgentPing(conn.GetServerID())

	response := map[string]interface{}{
		"type":   MessageTypeHello,
		"status": "success",
	}

	// 如果启用了加密，使用加密发送
	if conn.IsEncryptionEnabled() {
		return conn.WriteEncryptedJSON(response)
	}
	return conn.WriteJSON(response)
}

// HandleSystemInfo 处理系统信息消息
func (h *agentMessageHandler) HandleSystemInfo(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	systemData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("系统信息数据格式错误")
	}

	return h.saver.SaveSystemInfo(conn.GetServerID(), systemData)
}

// HandleMetrics 处理性能指标消息
func (h *agentMessageHandler) HandleMetrics(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	metricsData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("指标数据格式错误")
	}

	return h.saver.SaveMetrics(conn.GetServerID(), metricsData)
}

// HandleMemoryInfo 处理内存信息消息
func (h *agentMessageHandler) HandleMemoryInfo(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	memoryData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("内存信息数据格式错误")
	}

	return h.saver.SaveMemoryInfo(conn.GetServerID(), memoryData)
}

// HandleDiskInfo 处理磁盘信息消息
func (h *agentMessageHandler) HandleDiskInfo(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	diskData, ok := data["data"].([]interface{})
	if !ok {
		return errors.New("磁盘信息数据格式错误")
	}

	return h.saver.SaveDiskInfo(conn.GetServerID(), diskData)
}

// HandleDiskIO 处理磁盘IO消息
func (h *agentMessageHandler) HandleDiskIO(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	diskIOData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("磁盘IO数据格式错误")
	}

	return h.saver.SaveDiskIO(conn.GetServerID(), diskIOData)
}

// HandleNetworkInfo 处理网络信息消息
func (h *agentMessageHandler) HandleNetworkInfo(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	networkData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("网络信息数据格式错误")
	}

	return h.saver.SaveNetworkInfo(conn.GetServerID(), networkData)
}

// HandleSwapInfo 处理Swap内存消息
func (h *agentMessageHandler) HandleSwapInfo(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	swapData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("Swap数据格式错误")
	}

	return h.saver.SaveSwapInfo(conn.GetServerID(), swapData)
}

// HandleAgentConfig 处理Agent配置消息
func (h *agentMessageHandler) HandleAgentConfig(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	configData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("配置数据格式错误")
	}

	serverID := conn.GetServerID()
	serverRepo := repositories.NewServerRepository()

	// 构建更新数据
	updateData := make(map[string]interface{})

	if timezone, ok := configData["timezone"].(string); ok && timezone != "" {
		updateData["agent_timezone"] = timezone
	}
	if metricsInterval, ok := configData["metrics_interval"].(float64); ok && metricsInterval > 0 {
		updateData["agent_metrics_interval"] = int(metricsInterval)
	}
	if detailInterval, ok := configData["detail_interval"].(float64); ok && detailInterval > 0 {
		updateData["agent_detail_interval"] = int(detailInterval)
	}
	if systemInterval, ok := configData["system_interval"].(float64); ok && systemInterval > 0 {
		updateData["agent_system_interval"] = int(systemInterval)
	}
	if heartbeatInterval, ok := configData["heartbeat_interval"].(float64); ok && heartbeatInterval > 0 {
		updateData["agent_heartbeat_interval"] = int(heartbeatInterval)
	}
	if logPath, ok := configData["log_path"].(string); ok && logPath != "" {
		updateData["agent_log_path"] = logPath
	}

	if len(updateData) > 0 {
		if err := serverRepo.Update(serverID, updateData); err != nil {
			facades.Log().Channel("websocket").Errorf("更新Agent配置失败: %v", err)
			return fmt.Errorf("更新Agent配置失败: %w", err)
		}
		facades.Log().Channel("websocket").Infof("成功更新Agent配置: server_id=%s", serverID)
	}

	return nil
}

// frontendMessageHandler Frontend 消息处理器实现
type frontendMessageHandler struct {
	manager ConnectionManager
}

// NewFrontendMessageHandler 创建 Frontend 消息处理器
func NewFrontendMessageHandler(manager ConnectionManager) FrontendMessageHandler {
	return &frontendMessageHandler{
		manager: manager,
	}
}

// HandlePing 处理心跳消息
func (h *frontendMessageHandler) HandlePing(conn *FrontendConnection) error {
	h.manager.UpdateFrontendPing(conn.GetConnID())

	response := map[string]interface{}{
		"type":   MessageTypePong,
		"status": "success",
	}
	return conn.WriteJSON(response)
}

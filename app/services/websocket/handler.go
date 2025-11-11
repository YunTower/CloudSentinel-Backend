package websocket

import (
	"errors"
	"strings"

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
	// HandleVirtualMemory 处理虚拟内存消息
	HandleVirtualMemory(data map[string]interface{}, conn *AgentConnection) error
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

	// 更新连接信息
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

// HandleVirtualMemory 处理虚拟内存消息
func (h *agentMessageHandler) HandleVirtualMemory(data map[string]interface{}, conn *AgentConnection) error {
	if conn.GetState() != StateAuthenticated {
		return errors.New("未认证")
	}

	vmData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("虚拟内存数据格式错误")
	}

	return h.saver.SaveVirtualMemory(conn.GetServerID(), vmData)
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

package controllers

import (
	"encoding/json"
	"errors"
	"goravel/app/services"
	nethttp "net/http"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
	"github.com/gorilla/websocket"
)

type WebSocketController struct {
	Upgrader websocket.Upgrader
}

func NewWebSocketController() *WebSocketController {
	return &WebSocketController{
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *nethttp.Request) bool {
				return true // 生产环境应该更严格检查
			},
		},
	}
}

// HandleAgentConnection 处理agent的WebSocket连接
func (c *WebSocketController) HandleAgentConnection(ctx http.Context) http.Response {
	// 升级HTTP连接为WebSocket
	conn, err := c.Upgrader.Upgrade(ctx.Response().Writer(), ctx.Request().Origin(), nil)
	if err != nil {
		facades.Log().Errorf("WebSocket升级失败: %v", err)
		return ctx.Response().String(http.StatusBadRequest, "WebSocket升级失败")
	}

	remoteAddr := ctx.Request().Ip()
	facades.Log().Infof("新的WebSocket连接来自: %s", remoteAddr)

	// 创建连接对象
	agentConn := &services.AgentConnection{
		Conn:       conn,
		LastPing:   time.Now(),
		IsAuth:     false,
		RemoteAddr: remoteAddr,
	}

	defer func() {
		if agentConn.IsAuth && agentConn.ServerID != "" {
			services.GetWebSocketService().Unregister(agentConn.ServerID)
		}
		conn.Close()
		facades.Log().Infof("WebSocket连接关闭: %s", remoteAddr)
	}()

	// 启动读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				facades.Log().Errorf("WebSocket读取错误: %v", err)
			}
			break
		}

		// 解析消息
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			facades.Log().Errorf("消息解析失败: %v", err)
			c.sendError(conn, "消息格式错误")
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			facades.Log().Error("消息缺少type字段")
			c.sendError(conn, "消息缺少type字段")
			continue
		}

		// 处理消息
		if err := c.handleMessage(msgType, msg, agentConn); err != nil {
			facades.Log().Errorf("处理消息失败 [%s]: %v", msgType, err)
			c.sendError(conn, err.Error())
		}
	}

	return nil
}

// handleMessage 处理不同类型的消息
func (c *WebSocketController) handleMessage(msgType string, data map[string]interface{}, conn *services.AgentConnection) error {
	switch msgType {
	case "auth":
		return c.handleAuth(data, conn)
	case "hello":
		return c.handleHeartbeat(conn)
	case "system_info":
		return c.handleSystemInfo(data, conn)
	case "metrics":
		return c.handleMetrics(data, conn)
	case "cpu_info":
		return c.handleCPUInfo(data, conn)
	case "memory_info":
		return c.handleMemoryInfo(data, conn)
	case "disk_info":
		return c.handleDiskInfo(data, conn)
	case "network_info":
		return c.handleNetworkInfo(data, conn)
	case "virtual_memory":
		return c.handleVirtualMemory(data, conn)
	default:
		facades.Log().Warning("未知的消息类型: " + msgType)
		return errors.New("未知的消息类型")
	}
}

// handleAuth 处理认证消息
func (c *WebSocketController) handleAuth(data map[string]interface{}, conn *services.AgentConnection) error {
	authData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("认证数据格式错误")
	}

	agentKey, ok := authData["key"].(string)
	if !ok || agentKey == "" {
		return errors.New("缺少agent key")
	}

	authType, _ := authData["type"].(string)
	if authType != "server" {
		return errors.New("不支持的认证类型")
	}

	// 验证agent key和IP并获取server_id
	clientIP := conn.RemoteAddr
	serverID, err := services.ValidateAgentAuth(agentKey, clientIP)
	if err != nil {
		return errors.New("认证失败: " + err.Error())
	}

	// 更新连接信息
	conn.ServerID = serverID
	conn.AgentKey = agentKey
	conn.IsAuth = true
	conn.LastPing = time.Now()

	// 注册连接
	services.GetWebSocketService().Register(serverID, conn)

	// 发送认证成功响应
	response := map[string]interface{}{
		"type":    "auth",
		"status":  "success",
		"message": "认证成功",
		"data": map[string]interface{}{
			"server_id": serverID,
		},
	}

	facades.Log().Infof("Agent认证成功: server_id=%s, remote=%s", serverID, conn.RemoteAddr)
	return conn.Conn.WriteJSON(response)
}

// handleHeartbeat 处理心跳消息
func (c *WebSocketController) handleHeartbeat(conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	services.GetWebSocketService().UpdatePing(conn.ServerID)

	response := map[string]interface{}{
		"type":   "hello",
		"status": "success",
	}
	return conn.Conn.WriteJSON(response)
}

// handleSystemInfo 处理系统信息消息
func (c *WebSocketController) handleSystemInfo(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	systemData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("系统信息数据格式错误")
	}

	return services.SaveSystemInfo(conn.ServerID, systemData)
}

// handleMetrics 处理性能指标消息
func (c *WebSocketController) handleMetrics(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	metricsData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("指标数据格式错误")
	}

	return services.SaveMetrics(conn.ServerID, metricsData)
}

// handleCPUInfo 处理CPU信息消息
func (c *WebSocketController) handleCPUInfo(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	cpuData, ok := data["data"].([]interface{})
	if !ok {
		return errors.New("CPU信息数据格式错误")
	}

	return services.SaveCPUInfo(conn.ServerID, cpuData)
}

// handleMemoryInfo 处理内存信息消息
func (c *WebSocketController) handleMemoryInfo(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	memoryData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("内存信息数据格式错误")
	}

	return services.SaveMemoryInfo(conn.ServerID, memoryData)
}

// handleDiskInfo 处理磁盘信息消息
func (c *WebSocketController) handleDiskInfo(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	diskData, ok := data["data"].([]interface{})
	if !ok {
		return errors.New("磁盘信息数据格式错误")
	}

	return services.SaveDiskInfo(conn.ServerID, diskData)
}

// handleNetworkInfo 处理网络信息消息
func (c *WebSocketController) handleNetworkInfo(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	networkData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("网络信息数据格式错误")
	}

	return services.SaveNetworkInfo(conn.ServerID, networkData)
}

// handleVirtualMemory 处理虚拟内存消息
func (c *WebSocketController) handleVirtualMemory(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	vmData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("虚拟内存数据格式错误")
	}

	return services.SaveVirtualMemory(conn.ServerID, vmData)
}

// sendError 发送错误消息
func (c *WebSocketController) sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":    "error",
		"status":  "error",
		"message": message,
	}
	conn.WriteJSON(response)
}


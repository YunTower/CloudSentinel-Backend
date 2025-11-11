package controllers

import (
	"encoding/json"
	"errors"
	"goravel/app/services"
	"net"
	nethttp "net/http"
	"strings"
	"time"

	"github.com/google/uuid"
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
		facades.Log().Channel("websocket").Errorf("WebSocket升级失败: %v", err)
		return ctx.Response().String(http.StatusBadRequest, "WebSocket升级失败")
	}

	remoteAddr := c.extractIPFromAddr(conn.RemoteAddr())
	facades.Log().Channel("websocket").Infof("新的WebSocket连接来自: %s", remoteAddr)

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
		facades.Log().Channel("websocket").Infof("WebSocket连接关闭: %s", remoteAddr)
	}()

	// 启动读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				facades.Log().Channel("websocket").Errorf("WebSocket读取错误: %v", err)
			}
			break
		}

		// 解析消息
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			facades.Log().Channel("websocket").Warningf("消息解析失败: %v", err)
			c.sendError(conn, "消息格式错误")
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			facades.Log().Channel("websocket").Warning("消息缺少type字段")
			c.sendError(conn, "消息缺少type字段")
			continue
		}

		// 检查连接是否已被标记为关闭（可能被新连接替换）
		agentConn.Mutex.Lock()
		closed := agentConn.Closed
		agentConn.Mutex.Unlock()

		// 如果连接已被标记为关闭，停止处理消息
		if closed {
			facades.Log().Channel("websocket").Debugf("连接已被新连接替换，停止处理消息")
			break
		}

		// 处理消息
		if err := c.handleMessage(msgType, msg, agentConn); err != nil {
			// 再次检查连接状态
			agentConn.Mutex.Lock()
			closed = agentConn.Closed
			agentConn.Mutex.Unlock()

			// 如果连接已被关闭，不发送错误消息
			if closed {
				facades.Log().Channel("websocket").Debugf("连接已被关闭，跳过错误响应")
				break
			}

			if msgType == "auth" {
				facades.Log().Channel("websocket").Warningf("处理消息失败 [%s]: %v", msgType, err)
			} else {
				facades.Log().Channel("websocket").Errorf("处理消息失败 [%s]: %v", msgType, err)
			}
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
	case "memory_info":
		return c.handleMemoryInfo(data, conn)
	case "disk_info":
		return c.handleDiskInfo(data, conn)
	case "disk_io":
		return c.handleDiskIO(data, conn)
	case "network_info":
		return c.handleNetworkInfo(data, conn)
	case "virtual_memory":
		return c.handleVirtualMemory(data, conn)
	default:
		facades.Log().Channel("websocket").Warning("未知的消息类型: " + msgType)
		return nil
	}
}

// handleAuth 处理认证消息
func (c *WebSocketController) handleAuth(data map[string]interface{}, conn *services.AgentConnection) error {
	authData, ok := data["data"].(map[string]interface{})
	if !ok {
		// 认证数据格式错误
		facades.Log().Channel("websocket").Warning("认证数据格式错误")
		return errors.New("认证数据格式错误")
	}

	agentKey, ok := authData["key"].(string)
	if !ok || agentKey == "" {
		// 缺少 agent key
		facades.Log().Channel("websocket").Warningf("认证失败: 缺少agent key (IP: %s)", conn.RemoteAddr)
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
		facades.Log().Channel("websocket").Warningf("认证失败: 不支持的认证类型 %s (IP: %s, key: %s)", authType, conn.RemoteAddr, keyPreview)
		return errors.New("不支持的认证类型")
	}

	// 验证agent key和IP并获取server_id
	clientIP := conn.RemoteAddr
	keyPreview := agentKey
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8] + "..."
	}
	facades.Log().Channel("websocket").Infof("尝试认证: IP=%s, key=%s (完整长度: %d)", clientIP, keyPreview, len(agentKey))

	serverID, err := services.ValidateAgentAuth(agentKey, clientIP)
	if err != nil {
		facades.Log().Channel("websocket").Warningf("认证失败: %v (IP: %s, key: %s)", err, clientIP, keyPreview)
		return errors.New("认证失败: " + err.Error())
	}

	// 认证成功后，如果当前IP是127.0.0.1，使用服务器记录中的IP作为真实IP
	// 因为本地连接可能是通过代理、隧道或同一台机器，无法获取真实IP
	if strings.HasPrefix(clientIP, "127.0.0.1") || strings.HasPrefix(clientIP, "::1") || strings.HasPrefix(clientIP, "localhost") {
		var serverRecord []map[string]interface{}
		if err := facades.Orm().Query().Table("servers").
			Select("ip").
			Where("id", serverID).
			Get(&serverRecord); err == nil && len(serverRecord) > 0 {
			if serverIP, ok := serverRecord[0]["ip"].(string); ok && serverIP != "" && serverIP != "127.0.0.1" {
				// 使用服务器记录中的IP更新RemoteAddr（这是创建服务器时记录的真实IP）
				conn.RemoteAddr = serverIP
				facades.Log().Channel("websocket").Infof("检测到本地连接，使用服务器记录中的IP: %s (原连接IP: %s)", serverIP, clientIP)
			}
		}
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

	facades.Log().Channel("websocket").Infof("Agent认证成功: server_id=%s, remote=%s", serverID, conn.RemoteAddr)
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

// handleDiskIO 处理磁盘IO消息
func (c *WebSocketController) handleDiskIO(data map[string]interface{}, conn *services.AgentConnection) error {
	if !conn.IsAuth {
		return errors.New("未认证")
	}

	diskIOData, ok := data["data"].(map[string]interface{})
	if !ok {
		return errors.New("磁盘IO数据格式错误")
	}

	return services.SaveDiskIO(conn.ServerID, diskIOData)
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

// HandleFrontendConnection 处理前端的WebSocket连接
func (c *WebSocketController) HandleFrontendConnection(ctx http.Context) http.Response {
	// 先升级HTTP连接为WebSocket（必须在验证之前升级）
	conn, err := c.Upgrader.Upgrade(ctx.Response().Writer(), ctx.Request().Origin(), nil)
	if err != nil {
		facades.Log().Channel("websocket").Errorf("前端WebSocket升级失败: %v", err)
		return ctx.Response().String(http.StatusBadRequest, "WebSocket升级失败")
	}

	remoteAddr := c.getClientIPFromConn(conn, ctx)
	connID := uuid.New().String()
	facades.Log().Channel("websocket").Infof("新的前端WebSocket连接来自: %s (连接ID: %s)", remoteAddr, connID)

	// 从URL参数或Header获取token
	var token string

	// 尝试从URL查询参数获取token（通过原始HTTP请求）
	req := ctx.Request().Origin()
	if req != nil {
		token = req.URL.Query().Get("token")
	}

	// 如果URL参数中没有，尝试从Header获取
	if token == "" {
		authHeader := ctx.Request().Header("Authorization")
		if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	// 验证token（在升级后进行）
	var userID string

	if token != "" {
		// 解析和验证token
		payload, err := facades.Auth(ctx).Parse(token)
		if err != nil {
			facades.Log().Channel("websocket").Warningf("前端WebSocket token验证失败: %v", err)
			// 发送错误消息并关闭连接
			c.sendError(conn, "Token无效或已过期")
			conn.Close()
			return nil
		}
		userID = payload.Key
		facades.Log().Channel("websocket").Infof("前端WebSocket认证成功: 用户ID=%s (连接ID: %s)", userID, connID)
	} else {
		facades.Log().Channel("websocket").Warning("前端WebSocket连接缺少token")
		// 发送错误消息并关闭连接
		c.sendError(conn, "缺少认证token")
		conn.Close()
		return nil
	}

	// 创建前端连接对象
	frontendConn := &services.FrontendConnection{
		Conn:       conn,
		LastPing:   time.Now(),
		RemoteAddr: remoteAddr,
	}

	// 注册连接
	services.GetWebSocketService().RegisterFrontend(connID, frontendConn)

	defer func() {
		services.GetWebSocketService().UnregisterFrontend(connID)
		conn.Close()
		facades.Log().Channel("websocket").Infof("前端WebSocket连接关闭: %s (连接ID: %s)", remoteAddr, connID)
	}()

	// 启动读取消息循环
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				facades.Log().Channel("websocket").Errorf("前端WebSocket读取错误: %v", err)
			}
			break
		}

		// 解析消息
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			facades.Log().Channel("websocket").Warningf("前端消息解析失败: %v", err)
			c.sendError(conn, "消息格式错误")
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok {
			facades.Log().Channel("websocket").Warning("前端消息缺少type字段")
			c.sendError(conn, "消息缺少type字段")
			continue
		}

		// 处理心跳消息
		if msgType == "ping" {
			services.GetWebSocketService().UpdateFrontendPing(connID)
			response := map[string]interface{}{
				"type":   "pong",
				"status": "success",
			}
			if err := conn.WriteJSON(response); err != nil {
				facades.Log().Channel("websocket").Errorf("发送pong消息失败: %v", err)
				break
			}
		}
	}

	return nil
}

// sendError 发送错误消息
func (c *WebSocketController) sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":    "error",
		"status":  "error",
		"message": message,
	}
	// 忽略发送错误，因为连接可能已经关闭（例如被新连接替换）
	_ = conn.WriteJSON(response)
}

// extractIPFromAddr 从 net.Addr 中提取IP地址
func (c *WebSocketController) extractIPFromAddr(addr net.Addr) string {
	if addr == nil {
		return "unknown"
	}
	return c.extractIPFromAddrString(addr.String())
}

// extractIPFromAddrString 从地址字符串中提取IP地址
func (c *WebSocketController) extractIPFromAddrString(addrStr string) string {
	if addrStr == "" {
		return "unknown"
	}

	// 处理 IPv6 地址
	if strings.Contains(addrStr, "[") {
		// IPv6 格式：[IP]:Port
		start := strings.Index(addrStr, "[")
		end := strings.Index(addrStr, "]")
		if start != -1 && end != -1 && end > start {
			return addrStr[start+1 : end]
		}
	} else {
		// IPv4 格式：IP:Port
		parts := strings.Split(addrStr, ":")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return addrStr
}

// getClientIPFromConn 获取客户端真实IP地址
func (c *WebSocketController) getClientIPFromConn(conn *websocket.Conn, ctx http.Context) string {
	// 检查 X-Forwarded-For 头
	if xff := ctx.Request().Header("X-Forwarded-For"); xff != "" {
		// 取第一个IP
		if len(xff) > 0 {
			ips := strings.Split(xff, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if ip != "" {
					return ip
				}
			}
		}
	}

	// 检查 X-Real-Ip 头
	if xri := ctx.Request().Header("X-Real-Ip"); xri != "" {
		return xri
	}

	// 检查 X-Forwarded 头
	if xf := ctx.Request().Header("X-Forwarded"); xf != "" {
		if strings.HasPrefix(xf, "for=") {
			parts := strings.Split(xf, ";")
			if len(parts) > 0 {
				forPart := strings.TrimPrefix(parts[0], "for=")
				if forPart != "" {
					return strings.TrimSpace(forPart)
				}
			}
		}
	}

	// 从WebSocket连接对象获取远程地址
	if conn != nil {
		return c.extractIPFromAddr(conn.RemoteAddr())
	}

	return ctx.Request().Ip()
}

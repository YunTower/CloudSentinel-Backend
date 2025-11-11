package controllers

import (
	"encoding/json"
	"goravel/app/services"
	ws "goravel/app/services/websocket"

	"github.com/google/uuid"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
	"github.com/gorilla/websocket"
)

type WebSocketController struct {
	upgrader        *ws.Upgrader
	manager         ws.ConnectionManager
	agentHandler    ws.AgentMessageHandler
	frontendHandler ws.FrontendMessageHandler
	config          *ws.Config
}

func NewWebSocketController() *WebSocketController {
	// 创建配置
	config := ws.DefaultConfig()

	// 创建升级器
	upgrader := ws.NewUpgrader(config)

	// 创建连接管理器
	manager := ws.NewConnectionManager()

	// 创建消息处理器
	agentHandler := ws.NewAgentMessageHandler(manager, services.GetAgentAuthValidator(), services.GetAgentDataSaver())
	frontendHandler := ws.NewFrontendMessageHandler(manager)

	return &WebSocketController{
		upgrader:        upgrader,
		manager:         manager,
		agentHandler:    agentHandler,
		frontendHandler: frontendHandler,
		config:          config,
	}
}

// HandleAgentConnection 处理agent的WebSocket连接
func (c *WebSocketController) HandleAgentConnection(ctx http.Context) http.Response {
	// 升级HTTP连接为WebSocket
	conn, err := c.upgrader.Upgrade(ctx.Response().Writer(), ctx.Request().Origin(), nil)
	if err != nil {
		facades.Log().Channel("websocket").Errorf("WebSocket升级失败: %v", err)
		return ctx.Response().String(http.StatusBadRequest, "WebSocket升级失败")
	}

	remoteAddr := c.upgrader.ExtractIPFromAddr(conn.RemoteAddr())
	facades.Log().Channel("websocket").Infof("新的WebSocket连接来自: %s", remoteAddr)

	// 创建连接对象
	agentConn := ws.NewAgentConnection(conn, c.config)
	agentConn.SetRemoteAddr(remoteAddr)

	defer func() {
		if agentConn.GetState() == ws.StateAuthenticated && agentConn.GetServerID() != "" {
			c.manager.UnregisterAgent(agentConn.GetServerID())
		}
		agentConn.Close()
		facades.Log().Channel("websocket").Infof("WebSocket连接关闭: %s", remoteAddr)
	}()

	// 启动读取消息循环
	for {
		// 检查连接是否已关闭
		if agentConn.IsClosed() {
			facades.Log().Channel("websocket").Debugf("连接已被新连接替换，停止处理消息")
			break
		}

		// 读取消息（使用超时控制）
		_, message, err := agentConn.ReadMessage()
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

		// 再次检查连接状态
		if agentConn.IsClosed() {
			facades.Log().Channel("websocket").Debugf("连接已被关闭，跳过消息处理")
			break
		}

		// 处理消息
		if err := c.handleAgentMessage(msgType, msg, agentConn); err != nil {
			// 如果连接已被关闭，不发送错误消息
			if agentConn.IsClosed() {
				facades.Log().Channel("websocket").Debugf("连接已被关闭，跳过错误响应")
				break
			}

			if msgType == ws.MessageTypeAuth {
				facades.Log().Channel("websocket").Warningf("处理消息失败 [%s]: %v", msgType, err)
			} else {
				facades.Log().Channel("websocket").Errorf("处理消息失败 [%s]: %v", msgType, err)
			}
			c.sendError(conn, err.Error())
		}
	}

	return nil
}

// handleAgentMessage 处理不同类型的消息
func (c *WebSocketController) handleAgentMessage(msgType string, data map[string]interface{}, conn *ws.AgentConnection) error {
	switch msgType {
	case ws.MessageTypeAuth:
		return c.agentHandler.HandleAuth(data, conn)
	case ws.MessageTypeHello:
		return c.agentHandler.HandleHeartbeat(conn)
	case ws.MessageTypeSystemInfo:
		return c.agentHandler.HandleSystemInfo(data, conn)
	case ws.MessageTypeMetrics:
		return c.agentHandler.HandleMetrics(data, conn)
	case ws.MessageTypeMemoryInfo:
		return c.agentHandler.HandleMemoryInfo(data, conn)
	case ws.MessageTypeDiskInfo:
		return c.agentHandler.HandleDiskInfo(data, conn)
	case ws.MessageTypeDiskIO:
		return c.agentHandler.HandleDiskIO(data, conn)
	case ws.MessageTypeNetworkInfo:
		return c.agentHandler.HandleNetworkInfo(data, conn)
	case ws.MessageTypeVirtualMemory:
		return c.agentHandler.HandleVirtualMemory(data, conn)
	default:
		facades.Log().Channel("websocket").Warning("未知的消息类型: " + msgType)
		return nil
	}
}

// HandleFrontendConnection 处理前端的WebSocket连接
func (c *WebSocketController) HandleFrontendConnection(ctx http.Context) http.Response {
	// 先升级HTTP连接为WebSocket（必须在验证之前升级）
	conn, err := c.upgrader.Upgrade(ctx.Response().Writer(), ctx.Request().Origin(), nil)
	if err != nil {
		facades.Log().Channel("websocket").Errorf("前端WebSocket升级失败: %v", err)
		return ctx.Response().String(http.StatusBadRequest, "WebSocket升级失败")
	}

	remoteAddr := c.upgrader.GetClientIPFromConn(conn, ctx)
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
	frontendConn := ws.NewFrontendConnection(conn, c.config)
	frontendConn.SetConnID(connID)
	frontendConn.SetUserID(userID)
	frontendConn.SetRemoteAddr(remoteAddr)

	// 注册连接
	if err := c.manager.RegisterFrontend(connID, frontendConn); err != nil {
		facades.Log().Channel("websocket").Errorf("注册前端连接失败: %v", err)
		conn.Close()
		return nil
	}

	defer func() {
		c.manager.UnregisterFrontend(connID)
		frontendConn.Close()
		facades.Log().Channel("websocket").Infof("前端WebSocket连接关闭: %s (连接ID: %s)", remoteAddr, connID)
	}()

	// 启动读取消息循环
	for {
		// 检查连接是否已关闭
		if frontendConn.IsClosed() {
			break
		}

		// 读取消息（使用超时控制）
		_, message, err := frontendConn.ReadMessage()
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
		if msgType == ws.MessageTypePing {
			if err := c.frontendHandler.HandlePing(frontendConn); err != nil {
				facades.Log().Channel("websocket").Errorf("处理ping消息失败: %v", err)
				break
			}
		}
	}

	return nil
}

// sendError 发送错误消息
func (c *WebSocketController) sendError(conn *websocket.Conn, message string) {
	response := map[string]interface{}{
		"type":    ws.MessageTypeError,
		"status":  "error",
		"message": message,
	}
	// 忽略发送错误，因为连接可能已经关闭（例如被新连接替换）
	_ = conn.WriteJSON(response)
}

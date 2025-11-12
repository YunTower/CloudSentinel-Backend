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

	// 使用全局的 WebSocket 服务单例，确保连接管理器实例一致
	wsService := services.GetWebSocketService()
	manager := wsService.GetManager()

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

	// 发送认证成功消息
	authSuccessMsg := map[string]interface{}{
		"type":    "auth_success",
		"status":  "success",
		"message": "连接已建立",
		"data": map[string]interface{}{
			"conn_id": connID,
			"user_id": userID,
		},
	}
	if err := frontendConn.WriteJSON(authSuccessMsg); err != nil {
		facades.Log().Channel("websocket").Errorf("发送认证成功消息失败: %v", err)
	}

	// 检查是否有 agent 连接，并发送连接状态
	agentCount := c.manager.GetAgentConnectionCount()
	facades.Log().Channel("websocket").Infof("当前 agent 连接数: %d", agentCount)

	// 发送连接状态消息，告知前端当前 agent 连接数
	connectionStatusMsg := map[string]interface{}{
		"type":    "connection_status",
		"status":  "success",
		"message": "连接状态",
		"data": map[string]interface{}{
			"agent_count": agentCount,
		},
	}
	if err := frontendConn.WriteJSON(connectionStatusMsg); err != nil {
		facades.Log().Channel("websocket").Errorf("发送连接状态消息失败: %v", err)
	}

	// 推送所有在线服务器的初始状态数据
	c.pushInitialServerStates(frontendConn)

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

// pushInitialServerStates 推送所有在线服务器的初始状态数据
func (c *WebSocketController) pushInitialServerStates(frontendConn *ws.FrontendConnection) {
	// 检查连接是否已关闭
	if frontendConn.IsClosed() {
		facades.Log().Channel("websocket").Warning("前端连接已关闭，跳过初始数据推送")
		return
	}

	// 查询所有在线服务器
	var servers []map[string]interface{}
	err := facades.Orm().Query().Table("servers").
		Select("id", "boot_time").
		Where("status", "online").
		Get(&servers)

	if err != nil {
		facades.Log().Channel("websocket").Errorf("查询在线服务器失败: %v", err)
		return
	}

	if len(servers) == 0 {
		facades.Log().Channel("websocket").Debug("没有在线服务器，跳过初始数据推送")
		return
	}

	facades.Log().Channel("websocket").Infof("开始推送 %d 个在线服务器的初始状态", len(servers))

	// 为每个服务器推送最新状态
	for _, server := range servers {
		serverID, ok := server["id"].(string)
		if !ok {
			continue
		}

		// 检查连接是否已关闭
		if frontendConn.IsClosed() {
			facades.Log().Channel("websocket").Warning("前端连接已关闭，停止推送初始数据")
			return
		}

		// 获取最新指标数据
		var latestMetrics []map[string]interface{}
		err := facades.Orm().Query().Table("server_metrics").
			Select("cpu_usage", "memory_usage", "disk_usage", "network_upload", "network_download").
			Where("server_id", serverID).
			OrderBy("timestamp", "desc").
			Limit(1).
			Get(&latestMetrics)

		if err != nil {
			facades.Log().Channel("websocket").Warningf("查询服务器 %s 的指标数据失败: %v", serverID, err)
			continue
		}

		// 如果没有指标数据，跳过该服务器
		if len(latestMetrics) == 0 {
			facades.Log().Channel("websocket").Debugf("服务器 %s 没有指标数据，跳过", serverID)
			continue
		}

		metric := latestMetrics[0]

		// 计算运行时间
		uptimeStr := services.CalculateUptime(server["boot_time"])

		// 格式化数值为两位小数
		cpuUsage := services.FormatMetricValue(metric["cpu_usage"])
		memoryUsage := services.FormatMetricValue(metric["memory_usage"])
		diskUsage := services.FormatMetricValue(metric["disk_usage"])
		networkUpload := services.FormatMetricValue(metric["network_upload"])
		networkDownload := services.FormatMetricValue(metric["network_download"])

		// 构造并推送 metrics_update 消息
		message := map[string]interface{}{
			"type": "metrics_update",
			"data": map[string]interface{}{
				"server_id": serverID,
				"metrics": map[string]interface{}{
					"cpu_usage":        cpuUsage,
					"memory_usage":     memoryUsage,
					"disk_usage":       diskUsage,
					"network_upload":   networkUpload,
					"network_download": networkDownload,
				},
				"uptime": uptimeStr,
			},
		}

		if err := frontendConn.WriteJSON(message); err != nil {
			facades.Log().Channel("websocket").Errorf("推送服务器 %s 的初始状态失败: %v", serverID, err)
			// 如果连接已关闭，停止推送
			if frontendConn.IsClosed() {
				return
			}
		} else {
			facades.Log().Channel("websocket").Debugf("成功推送服务器 %s 的初始状态", serverID)
		}
	}

	facades.Log().Channel("websocket").Info("初始服务器状态推送完成")
}

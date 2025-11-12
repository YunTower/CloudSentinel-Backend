package services

import (
	"context"
	"sync"
	"time"

	ws "goravel/app/services/websocket"

	"github.com/gorilla/websocket"
)

// AgentConnection 表示一个agent连接（保持向后兼容）
type AgentConnection struct {
	Conn       *websocket.Conn
	ServerID   string
	AgentKey   string
	LastPing   time.Time
	Mutex      sync.Mutex
	IsAuth     bool
	RemoteAddr string
	Closed     bool // 标记连接是否已被关闭
}

// FrontendConnection 表示一个前端连接（保持向后兼容）
type FrontendConnection struct {
	Conn       *websocket.Conn
	LastPing   time.Time
	Mutex      sync.Mutex
	RemoteAddr string
}

// WebSocketService 管理所有WebSocket连接（保持向后兼容的公共接口）
type WebSocketService struct {
	manager ws.ConnectionManager
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once
}

var wsService *WebSocketService
var serviceOnce sync.Once

// GetWebSocketService 获取WebSocket服务单例
func GetWebSocketService() *WebSocketService {
	serviceOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		wsService = &WebSocketService{
			manager: ws.NewConnectionManager(),
			ctx:     ctx,
			cancel:  cancel,
		}
		// 启动心跳检测
		go wsService.manager.StartHeartbeatChecker(ctx)
	})
	return wsService
}

// GetManager 获取连接管理器
func (s *WebSocketService) GetManager() ws.ConnectionManager {
	return s.manager
}

// Register 注册新的agent连接（保持向后兼容）
func (s *WebSocketService) Register(serverID string, conn *AgentConnection) {
	// 转换为新的连接类型
	newConn := convertToNewAgentConnection(conn)
	if newConn != nil {
		_ = s.manager.RegisterAgent(serverID, newConn)
		// 更新旧连接对象以保持兼容
		updateOldAgentConnection(conn, newConn)
	}
}

// Unregister 注销agent连接（保持向后兼容）
func (s *WebSocketService) Unregister(serverID string) {
	s.manager.UnregisterAgent(serverID)
}

// GetConnection 获取指定服务器的连接（保持向后兼容）
func (s *WebSocketService) GetConnection(serverID string) (*AgentConnection, bool) {
	newConn, exists := s.manager.GetAgentConnection(serverID)
	if !exists {
		return nil, false
	}
	// 转换为旧类型
	return convertToOldAgentConnection(newConn), true
}

// GetAllConnections 获取所有连接（保持向后兼容）
func (s *WebSocketService) GetAllConnections() map[string]*AgentConnection {
	newConns := s.manager.GetAllAgentConnections()
	result := make(map[string]*AgentConnection)
	for k, v := range newConns {
		result[k] = convertToOldAgentConnection(v)
	}
	return result
}

// UpdatePing 更新最后ping时间（保持向后兼容）
func (s *WebSocketService) UpdatePing(serverID string) {
	s.manager.UpdateAgentPing(serverID)
}

// SendMessage 向指定服务器发送消息（保持向后兼容）
func (s *WebSocketService) SendMessage(serverID string, message interface{}) error {
	return s.manager.SendToAgent(serverID, message)
}

// Broadcast 向所有连接广播消息（保持向后兼容）
func (s *WebSocketService) Broadcast(message interface{}) {
	s.manager.BroadcastToAgents(message)
}

// GetConnectionCount 获取当前连接数（保持向后兼容）
func (s *WebSocketService) GetConnectionCount() int {
	return s.manager.GetAgentConnectionCount()
}

// RegisterFrontend 注册新的前端连接（保持向后兼容）
func (s *WebSocketService) RegisterFrontend(connID string, conn *FrontendConnection) {
	// 转换为新的连接类型
	newConn := convertToNewFrontendConnection(conn)
	if newConn != nil {
		newConn.SetConnID(connID)
		_ = s.manager.RegisterFrontend(connID, newConn)
		// 更新旧连接对象以保持兼容
		updateOldFrontendConnection(conn, newConn)
	}
}

// UnregisterFrontend 注销前端连接（保持向后兼容）
func (s *WebSocketService) UnregisterFrontend(connID string) {
	s.manager.UnregisterFrontend(connID)
}

// BroadcastToFrontend 向前端连接广播消息（保持向后兼容）
func (s *WebSocketService) BroadcastToFrontend(message interface{}) {
	s.manager.BroadcastToFrontend(message)
}

// UpdateFrontendPing 更新前端连接的最后ping时间（保持向后兼容）
func (s *WebSocketService) UpdateFrontendPing(connID string) {
	s.manager.UpdateFrontendPing(connID)
}

// GetFrontendConnectionCount 获取当前前端连接数（保持向后兼容）
func (s *WebSocketService) GetFrontendConnectionCount() int {
	return s.manager.GetFrontendConnectionCount()
}

// 转换函数：旧类型 -> 新类型
func convertToNewAgentConnection(oldConn *AgentConnection) *ws.AgentConnection {
	if oldConn == nil || oldConn.Conn == nil {
		return nil
	}
	config := ws.DefaultConfig()
	newConn := ws.NewAgentConnection(oldConn.Conn, config)
	newConn.SetServerID(oldConn.ServerID)
	newConn.SetAgentKey(oldConn.AgentKey)
	newConn.SetRemoteAddr(oldConn.RemoteAddr)
	if oldConn.IsAuth {
		newConn.SetState(ws.StateAuthenticated)
	}
	if oldConn.Closed {
		newConn.SetState(ws.StateClosed)
	}
	// 同步 LastPing
	newConn.UpdateLastPing()
	return newConn
}

func convertToNewFrontendConnection(oldConn *FrontendConnection) *ws.FrontendConnection {
	if oldConn == nil || oldConn.Conn == nil {
		return nil
	}
	config := ws.DefaultConfig()
	newConn := ws.NewFrontendConnection(oldConn.Conn, config)
	newConn.SetRemoteAddr(oldConn.RemoteAddr)
	newConn.UpdateLastPing()
	return newConn
}

// 转换函数：新类型 -> 旧类型
func convertToOldAgentConnection(newConn *ws.AgentConnection) *AgentConnection {
	if newConn == nil {
		return nil
	}
	oldConn := &AgentConnection{
		Conn:       newConn.GetConn(),
		ServerID:   newConn.GetServerID(),
		AgentKey:   newConn.GetAgentKey(),
		LastPing:   newConn.GetLastPing(),
		RemoteAddr: newConn.GetRemoteAddr(),
		IsAuth:     newConn.GetState() == ws.StateAuthenticated,
		Closed:     newConn.IsClosed(),
	}
	return oldConn
}

func convertToOldFrontendConnection(newConn *ws.FrontendConnection) *FrontendConnection {
	if newConn == nil {
		return nil
	}
	oldConn := &FrontendConnection{
		Conn:       newConn.GetConn(),
		LastPing:   newConn.GetLastPing(),
		RemoteAddr: newConn.GetRemoteAddr(),
	}
	return oldConn
}

// 更新旧连接对象以保持兼容
func updateOldAgentConnection(oldConn *AgentConnection, newConn *ws.AgentConnection) {
	if oldConn == nil || newConn == nil {
		return
	}
	oldConn.Mutex.Lock()
	defer oldConn.Mutex.Unlock()
	oldConn.ServerID = newConn.GetServerID()
	oldConn.AgentKey = newConn.GetAgentKey()
	oldConn.RemoteAddr = newConn.GetRemoteAddr()
	oldConn.IsAuth = newConn.GetState() == ws.StateAuthenticated
	oldConn.Closed = newConn.IsClosed()
	oldConn.LastPing = newConn.GetLastPing()
}

func updateOldFrontendConnection(oldConn *FrontendConnection, newConn *ws.FrontendConnection) {
	if oldConn == nil || newConn == nil {
		return
	}
	oldConn.Mutex.Lock()
	defer oldConn.Mutex.Unlock()
	oldConn.RemoteAddr = newConn.GetRemoteAddr()
	oldConn.LastPing = newConn.GetLastPing()
}

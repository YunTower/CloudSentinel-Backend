package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/goravel/framework/facades"
)

// ConnectionManager 连接管理器接口
type ConnectionManager interface {
	// RegisterAgent 注册 agent 连接
	RegisterAgent(serverID string, conn *AgentConnection) error
	// UnregisterAgent 注销 agent 连接
	UnregisterAgent(serverID string)
	// GetAgentConnection 获取 agent 连接
	GetAgentConnection(serverID string) (*AgentConnection, bool)
	// GetAllAgentConnections 获取所有 agent 连接
	GetAllAgentConnections() map[string]*AgentConnection
	// UpdateAgentPing 更新 agent 心跳时间
	UpdateAgentPing(serverID string)
	// SendToAgent 向指定 agent 发送消息
	SendToAgent(serverID string, message interface{}) error
	// BroadcastToAgents 向所有 agent 广播消息
	BroadcastToAgents(message interface{})
	// RegisterFrontend 注册前端连接
	RegisterFrontend(connID string, conn *FrontendConnection) error
	// UnregisterFrontend 注销前端连接
	UnregisterFrontend(connID string)
	// GetFrontendConnection 获取前端连接
	GetFrontendConnection(connID string) (*FrontendConnection, bool)
	// GetAllFrontendConnections 获取所有前端连接
	GetAllFrontendConnections() map[string]*FrontendConnection
	// UpdateFrontendPing 更新前端心跳时间
	UpdateFrontendPing(connID string)
	// BroadcastToFrontend 向前端连接广播消息
	BroadcastToFrontend(message interface{})
	// GetAgentConnectionCount 获取 agent 连接数
	GetAgentConnectionCount() int
	// GetFrontendConnectionCount 获取前端连接数
	GetFrontendConnectionCount() int
	// StartHeartbeatChecker 启动心跳检测
	StartHeartbeatChecker(ctx context.Context)
}

// connectionManager 连接管理器实现
type connectionManager struct {
	agentConnections     map[string]*AgentConnection
	frontendConnections  map[string]*FrontendConnection
	agentMutex           sync.RWMutex
	frontendMutex        sync.RWMutex
	oldConnectionCloseDelay time.Duration
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager() ConnectionManager {
	return &connectionManager{
		agentConnections:        make(map[string]*AgentConnection),
		frontendConnections:     make(map[string]*FrontendConnection),
		oldConnectionCloseDelay: 2 * time.Second, // 旧连接关闭延迟
	}
}

// RegisterAgent 注册 agent 连接
func (m *connectionManager) RegisterAgent(serverID string, conn *AgentConnection) error {
	m.agentMutex.Lock()
	defer m.agentMutex.Unlock()

	// 如果已存在旧连接，先标记为已关闭，然后异步关闭
	if oldConn, exists := m.agentConnections[serverID]; exists {
		// 标记连接已关闭，避免继续处理消息
		oldConn.SetState(StateClosed)

		// 异步关闭连接，给当前消息处理充足的时间
		go func(oldConn *AgentConnection) {
			// 等待一段时间，确保所有待处理消息完成
			time.Sleep(m.oldConnectionCloseDelay)
			if err := oldConn.Close(); err != nil {
				facades.Log().Channel("websocket").Warningf("关闭服务器 %s 的旧连接失败: %v", serverID, err)
			} else {
				facades.Log().Channel("websocket").Infof("关闭服务器 %s 的旧连接", serverID)
			}
		}(oldConn)
	}

	m.agentConnections[serverID] = conn
	facades.Log().Channel("websocket").Infof("注册服务器连接: %s (来自 %s)", serverID, conn.GetRemoteAddr())
	return nil
}

// UnregisterAgent 注销 agent 连接
func (m *connectionManager) UnregisterAgent(serverID string) {
	m.agentMutex.Lock()
	defer m.agentMutex.Unlock()

	if conn, exists := m.agentConnections[serverID]; exists {
		conn.SetState(StateClosed)
		conn.Close()
		delete(m.agentConnections, serverID)
		facades.Log().Channel("websocket").Infof("注销服务器连接: %s", serverID)

		// 更新服务器状态为offline
		go func() {
			_, err := facades.Orm().Query().Table("servers").
				Where("id", serverID).
				Update(map[string]interface{}{
					"status":     "offline",
					"updated_at": time.Now().Unix(),
				})
			if err != nil {
				facades.Log().Channel("websocket").Errorf("更新服务器状态失败: %v", err)
			}
		}()
	}
}

// GetAgentConnection 获取 agent 连接
func (m *connectionManager) GetAgentConnection(serverID string) (*AgentConnection, bool) {
	m.agentMutex.RLock()
	defer m.agentMutex.RUnlock()
	conn, exists := m.agentConnections[serverID]
	return conn, exists
}

// GetAllAgentConnections 获取所有 agent 连接
func (m *connectionManager) GetAllAgentConnections() map[string]*AgentConnection {
	m.agentMutex.RLock()
	defer m.agentMutex.RUnlock()
	// 返回副本避免并发问题
	result := make(map[string]*AgentConnection)
	for k, v := range m.agentConnections {
		result[k] = v
	}
	return result
}

// UpdateAgentPing 更新 agent 心跳时间
func (m *connectionManager) UpdateAgentPing(serverID string) {
	conn, exists := m.GetAgentConnection(serverID)
	if exists {
		conn.UpdateLastPing()
	}
}

// SendToAgent 向指定 agent 发送消息
func (m *connectionManager) SendToAgent(serverID string, message interface{}) error {
	conn, exists := m.GetAgentConnection(serverID)
	if !exists {
		return ErrConnectionNotFound
	}

	if conn.IsClosed() {
		return ErrConnectionClosed
	}

	return conn.WriteJSON(message)
}

// BroadcastToAgents 向所有 agent 广播消息
func (m *connectionManager) BroadcastToAgents(message interface{}) {
	connections := m.GetAllAgentConnections()
	for serverID, conn := range connections {
		if conn.IsClosed() {
			continue
		}

		if err := conn.WriteJSON(message); err != nil {
			facades.Log().Channel("websocket").Errorf("向服务器 %s 发送消息失败: %v", serverID, err)
			go m.UnregisterAgent(serverID)
		}
	}
}

// RegisterFrontend 注册前端连接
func (m *connectionManager) RegisterFrontend(connID string, conn *FrontendConnection) error {
	m.frontendMutex.Lock()
	defer m.frontendMutex.Unlock()

	// 如果已存在旧连接，先关闭
	if oldConn, exists := m.frontendConnections[connID]; exists {
		oldConn.SetState(StateClosed)
		oldConn.Close()
		facades.Log().Channel("websocket").Infof("关闭前端连接 %s 的旧连接", connID)
	}

	m.frontendConnections[connID] = conn
	facades.Log().Channel("websocket").Infof("注册前端连接: %s (来自 %s)", connID, conn.GetRemoteAddr())
	return nil
}

// UnregisterFrontend 注销前端连接
func (m *connectionManager) UnregisterFrontend(connID string) {
	m.frontendMutex.Lock()
	defer m.frontendMutex.Unlock()

	if conn, exists := m.frontendConnections[connID]; exists {
		conn.SetState(StateClosed)
		conn.Close()
		delete(m.frontendConnections, connID)
		facades.Log().Channel("websocket").Infof("注销前端连接: %s", connID)
	}
}

// GetFrontendConnection 获取前端连接
func (m *connectionManager) GetFrontendConnection(connID string) (*FrontendConnection, bool) {
	m.frontendMutex.RLock()
	defer m.frontendMutex.RUnlock()
	conn, exists := m.frontendConnections[connID]
	return conn, exists
}

// GetAllFrontendConnections 获取所有前端连接
func (m *connectionManager) GetAllFrontendConnections() map[string]*FrontendConnection {
	m.frontendMutex.RLock()
	defer m.frontendMutex.RUnlock()
	// 返回副本避免并发问题
	result := make(map[string]*FrontendConnection)
	for k, v := range m.frontendConnections {
		result[k] = v
	}
	return result
}

// UpdateFrontendPing 更新前端心跳时间
func (m *connectionManager) UpdateFrontendPing(connID string) {
	conn, exists := m.GetFrontendConnection(connID)
	if exists {
		conn.UpdateLastPing()
	}
}

// BroadcastToFrontend 向前端连接广播消息
func (m *connectionManager) BroadcastToFrontend(message interface{}) {
	connections := m.GetAllFrontendConnections()
	for connID, conn := range connections {
		if conn.IsClosed() {
			continue
		}

		if err := conn.WriteJSON(message); err != nil {
			facades.Log().Channel("websocket").Errorf("向前端连接 %s 发送消息失败: %v", connID, err)
			go m.UnregisterFrontend(connID)
		}
	}
}

// GetAgentConnectionCount 获取 agent 连接数
func (m *connectionManager) GetAgentConnectionCount() int {
	m.agentMutex.RLock()
	defer m.agentMutex.RUnlock()
	return len(m.agentConnections)
}

// GetFrontendConnectionCount 获取前端连接数
func (m *connectionManager) GetFrontendConnectionCount() int {
	m.frontendMutex.RLock()
	defer m.frontendMutex.RUnlock()
	return len(m.frontendConnections)
}

// StartHeartbeatChecker 启动心跳检测
func (m *connectionManager) StartHeartbeatChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAgentHeartbeats()
		}
	}
}

// checkAgentHeartbeats 检查 agent 心跳
func (m *connectionManager) checkAgentHeartbeats() {
	now := time.Now()
	connections := m.GetAllAgentConnections()

	for serverID, conn := range connections {
		if conn.IsClosed() {
			continue
		}

		lastPing := conn.GetLastPing()
		// 超过60秒未收到心跳，断开连接
		if now.Sub(lastPing) > 60*time.Second {
			facades.Log().Channel("websocket").Warningf("服务器 %s 心跳超时，断开连接", serverID)
			m.UnregisterAgent(serverID)
		}
	}
}

// 错误定义
var (
	ErrConnectionNotFound = &ConnectionError{Message: "连接不存在"}
	ErrConnectionClosed  = &ConnectionError{Message: "连接已关闭"}
)

// ConnectionError 连接错误
type ConnectionError struct {
	Message string
}

func (e *ConnectionError) Error() string {
	return e.Message
}


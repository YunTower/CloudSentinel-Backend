package services

import (
	"errors"
	"sync"
	"time"

	"github.com/goravel/framework/facades"
	"github.com/gorilla/websocket"
)

// AgentConnection 表示一个agent连接
type AgentConnection struct {
	Conn       *websocket.Conn
	ServerID   string
	AgentKey   string
	LastPing   time.Time
	Mutex      sync.Mutex
	IsAuth     bool
	RemoteAddr string
}

// WebSocketService 管理所有WebSocket连接
type WebSocketService struct {
	connections map[string]*AgentConnection
	mutex       sync.RWMutex
}

var wsService *WebSocketService
var once sync.Once

// GetWebSocketService 获取WebSocket服务单例
func GetWebSocketService() *WebSocketService {
	once.Do(func() {
		wsService = &WebSocketService{
			connections: make(map[string]*AgentConnection),
		}
		// 启动心跳检测
		go wsService.startHeartbeatChecker()
	})
	return wsService
}

// Register 注册新的agent连接
func (s *WebSocketService) Register(serverID string, conn *AgentConnection) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	// 如果已存在旧连接，先关闭
	if oldConn, exists := s.connections[serverID]; exists {
		oldConn.Conn.Close()
		facades.Log().Infof("关闭服务器 %s 的旧连接", serverID)
	}
	
	s.connections[serverID] = conn
	facades.Log().Infof("注册服务器连接: %s (来自 %s)", serverID, conn.RemoteAddr)
}

// Unregister 注销agent连接
func (s *WebSocketService) Unregister(serverID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if conn, exists := s.connections[serverID]; exists {
		conn.Conn.Close()
		delete(s.connections, serverID)
		facades.Log().Infof("注销服务器连接: %s", serverID)
		
		// 更新服务器状态为offline
		go func() {
			_, err := facades.Orm().Query().Table("servers").
				Where("id", serverID).
				Update(map[string]interface{}{
					"status":     "offline",
					"updated_at": time.Now().Unix(),
				})
			if err != nil {
				facades.Log().Errorf("更新服务器状态失败: %v", err)
			}
		}()
	}
}

// GetConnection 获取指定服务器的连接
func (s *WebSocketService) GetConnection(serverID string) (*AgentConnection, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	conn, exists := s.connections[serverID]
	return conn, exists
}

// GetAllConnections 获取所有连接
func (s *WebSocketService) GetAllConnections() map[string]*AgentConnection {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	// 返回副本避免并发问题
	result := make(map[string]*AgentConnection)
	for k, v := range s.connections {
		result[k] = v
	}
	return result
}

// UpdatePing 更新最后ping时间
func (s *WebSocketService) UpdatePing(serverID string) {
	s.mutex.RLock()
	conn, exists := s.connections[serverID]
	s.mutex.RUnlock()
	
	if exists {
		conn.Mutex.Lock()
		conn.LastPing = time.Now()
		conn.Mutex.Unlock()
	}
}

// SendMessage 向指定服务器发送消息
func (s *WebSocketService) SendMessage(serverID string, message interface{}) error {
	conn, exists := s.GetConnection(serverID)
	if !exists {
		return errors.New("服务器连接不存在")
	}
	
	conn.Mutex.Lock()
	defer conn.Mutex.Unlock()
	
	return conn.Conn.WriteJSON(message)
}

// Broadcast 向所有连接广播消息
func (s *WebSocketService) Broadcast(message interface{}) {
	connections := s.GetAllConnections()
	for serverID, conn := range connections {
		conn.Mutex.Lock()
		err := conn.Conn.WriteJSON(message)
		conn.Mutex.Unlock()
		
		if err != nil {
			facades.Log().Errorf("向服务器 %s 发送消息失败: %v", serverID, err)
			go s.Unregister(serverID)
		}
	}
}

// startHeartbeatChecker 启动心跳检测
func (s *WebSocketService) startHeartbeatChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		now := time.Now()
		connections := s.GetAllConnections()
		
		for serverID, conn := range connections {
			conn.Mutex.Lock()
			lastPing := conn.LastPing
			conn.Mutex.Unlock()
			
			// 超过60秒未收到心跳，断开连接
			if now.Sub(lastPing) > 60*time.Second {
				facades.Log().Warning("服务器 " + serverID + " 心跳超时，断开连接")
				s.Unregister(serverID)
			}
		}
	}
}

// GetConnectionCount 获取当前连接数
func (s *WebSocketService) GetConnectionCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.connections)
}


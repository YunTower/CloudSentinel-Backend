package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// BaseConnection 基础连接实现
type BaseConnection struct {
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	state    ConnectionState
	mu       sync.RWMutex
	writeMu  sync.Mutex // 保护 WebSocket 写入操作，防止并发写入
	config   *Config
	lastPing time.Time
	muPing   sync.RWMutex
}

// NewBaseConnection 创建基础连接
func NewBaseConnection(conn *websocket.Conn, config *Config) *BaseConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &BaseConnection{
		conn:     conn,
		ctx:      ctx,
		cancel:   cancel,
		state:    StateConnecting,
		config:   config,
		lastPing: time.Now(),
	}
}

// GetConn 获取底层 WebSocket 连接
func (c *BaseConnection) GetConn() *websocket.Conn {
	return c.conn
}

// GetContext 获取连接的上下文
func (c *BaseConnection) GetContext() context.Context {
	return c.ctx
}

// GetState 获取连接状态
func (c *BaseConnection) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetState 设置连接状态（线程安全）
func (c *BaseConnection) SetState(state ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

// IsClosed 检查连接是否已关闭
func (c *BaseConnection) IsClosed() bool {
	return c.GetState() == StateClosed
}

// Close 关闭连接
func (c *BaseConnection) Close() error {
	c.SetState(StateClosed)
	c.cancel()
	return c.conn.Close()
}

// UpdateLastPing 更新最后心跳时间
func (c *BaseConnection) UpdateLastPing() {
	c.muPing.Lock()
	defer c.muPing.Unlock()
	c.lastPing = time.Now()
}

// GetLastPing 获取最后心跳时间
func (c *BaseConnection) GetLastPing() time.Time {
	c.muPing.RLock()
	defer c.muPing.RUnlock()
	return c.lastPing
}

// SetReadDeadline 设置读取超时
func (c *BaseConnection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline 设置写入超时
func (c *BaseConnection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// ReadMessage 读取消息（使用超时控制）
func (c *BaseConnection) ReadMessage() (messageType int, p []byte, err error) {
	if c.config != nil && c.config.ReadTimeout > 0 {
		deadline := time.Now().Add(c.config.ReadTimeout)
		if err := c.SetReadDeadline(deadline); err != nil {
			return 0, nil, err
		}
	}
	return c.conn.ReadMessage()
}

// WriteJSON 写入 JSON 消息
func (c *BaseConnection) WriteJSON(v interface{}) error {
	// 使用写锁保护，防止并发写入导致 panic
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// 检查连接是否已关闭
	if c.IsClosed() {
		return ErrConnectionClosed
	}

	if c.config != nil && c.config.WriteTimeout > 0 {
		deadline := time.Now().Add(c.config.WriteTimeout)
		if err := c.SetWriteDeadline(deadline); err != nil {
			return err
		}
	}
	return c.conn.WriteJSON(v)
}

// AgentConnection Agent 连接
type AgentConnection struct {
	*BaseConnection
	info *AgentConnectionInfo
	mu   sync.RWMutex
}

// NewAgentConnection 创建 Agent 连接
func NewAgentConnection(conn *websocket.Conn, config *Config) *AgentConnection {
	return &AgentConnection{
		BaseConnection: NewBaseConnection(conn, config),
		info: &AgentConnectionInfo{
			LastPing: time.Now(),
		},
	}
}

// GetServerID 获取服务器 ID
func (c *AgentConnection) GetServerID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.ServerID
}

// SetServerID 设置服务器 ID
func (c *AgentConnection) SetServerID(serverID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.ServerID = serverID
}

// GetAgentKey 获取 Agent Key
func (c *AgentConnection) GetAgentKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.AgentKey
}

// SetAgentKey 设置 Agent Key
func (c *AgentConnection) SetAgentKey(agentKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.AgentKey = agentKey
}

// GetRemoteAddr 获取远程地址
func (c *AgentConnection) GetRemoteAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.RemoteAddr
}

// SetRemoteAddr 设置远程地址
func (c *AgentConnection) SetRemoteAddr(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.RemoteAddr = addr
}

// GetInfo 获取连接信息
func (c *AgentConnection) GetInfo() *AgentConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 返回副本避免并发问题
	info := *c.info
	info.LastPing = c.GetLastPing()
	return &info
}

// FrontendConnection Frontend 连接
type FrontendConnection struct {
	*BaseConnection
	info *FrontendConnectionInfo
	mu   sync.RWMutex
}

// NewFrontendConnection 创建 Frontend 连接
func NewFrontendConnection(conn *websocket.Conn, config *Config) *FrontendConnection {
	return &FrontendConnection{
		BaseConnection: NewBaseConnection(conn, config),
		info: &FrontendConnectionInfo{
			LastPing: time.Now(),
		},
	}
}

// GetConnID 获取连接 ID
func (c *FrontendConnection) GetConnID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.ConnID
}

// SetConnID 设置连接 ID
func (c *FrontendConnection) SetConnID(connID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.ConnID = connID
}

// GetUserID 获取用户 ID
func (c *FrontendConnection) GetUserID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.UserID
}

// SetUserID 设置用户 ID
func (c *FrontendConnection) SetUserID(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.UserID = userID
}

// GetRemoteAddr 获取远程地址
func (c *FrontendConnection) GetRemoteAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.RemoteAddr
}

// SetRemoteAddr 设置远程地址
func (c *FrontendConnection) SetRemoteAddr(addr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.RemoteAddr = addr
}

// GetInfo 获取连接信息
func (c *FrontendConnection) GetInfo() *FrontendConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// 返回副本避免并发问题
	info := *c.info
	info.LastPing = c.GetLastPing()
	return &info
}

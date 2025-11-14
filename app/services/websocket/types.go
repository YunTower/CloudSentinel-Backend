package websocket

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

// ConnectionState 连接状态
type ConnectionState int

const (
	// StateConnecting 连接中（未认证）
	StateConnecting ConnectionState = iota
	// StateAuthenticated 已认证
	StateAuthenticated
	// StateClosed 已关闭
	StateClosed
)

// String 返回连接状态的字符串表示
func (s ConnectionState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateAuthenticated:
		return "authenticated"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// MessageType 消息类型常量
const (
	MessageTypeAuth          = "auth"
	MessageTypeHello         = "hello"
	MessageTypeSystemInfo    = "system_info"
	MessageTypeMetrics       = "metrics"
	MessageTypeMemoryInfo    = "memory_info"
	MessageTypeDiskInfo      = "disk_info"
	MessageTypeDiskIO      = "disk_io"
	MessageTypeNetworkInfo = "network_info"
	MessageTypeSwapInfo    = "swap_info"
	MessageTypePing        = "ping"
	MessageTypePong          = "pong"
	MessageTypeError         = "error"
)

// Connection 连接接口
type Connection interface {
	// GetConn 获取底层 WebSocket 连接
	GetConn() *websocket.Conn
	// GetContext 获取连接的上下文
	GetContext() context.Context
	// GetState 获取连接状态
	GetState() ConnectionState
	// SetState 设置连接状态（线程安全）
	SetState(state ConnectionState)
	// IsClosed 检查连接是否已关闭
	IsClosed() bool
	// Close 关闭连接
	Close() error
}

// AgentConnectionInfo Agent 连接信息
type AgentConnectionInfo struct {
	ServerID   string
	AgentKey   string
	RemoteAddr string
	LastPing   time.Time
}

// FrontendConnectionInfo Frontend 连接信息
type FrontendConnectionInfo struct {
	ConnID     string
	UserID     string
	RemoteAddr string
	LastPing   time.Time
}

// Config WebSocket 配置
type Config struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r interface{}) bool
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PongWait        time.Duration
	PingPeriod      time.Duration
	MaxMessageSize  int64
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     nil, // 默认允许所有来源
		ReadTimeout:     90 * time.Second,
		WriteTimeout:    10 * time.Second,
		PongWait:        60 * time.Second,
		PingPeriod:      30 * time.Second,
		MaxMessageSize:  512,
	}
}

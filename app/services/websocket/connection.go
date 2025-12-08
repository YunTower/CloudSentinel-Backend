package websocket

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// 复制 SessionKey 避免外部修改
	if info.SessionKey != nil {
		info.SessionKey = make([]byte, len(c.info.SessionKey))
		copy(info.SessionKey, c.info.SessionKey)
	}
	return &info
}

// SetSessionKey 设置 AES 会话密钥
func (c *AgentConnection) SetSessionKey(key []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 复制密钥避免外部修改
	if key != nil {
		c.info.SessionKey = make([]byte, len(key))
		copy(c.info.SessionKey, key)
	} else {
		c.info.SessionKey = nil
	}
}

// GetSessionKey 获取 AES 会话密钥（返回副本）
func (c *AgentConnection) GetSessionKey() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.info.SessionKey == nil {
		return nil
	}
	key := make([]byte, len(c.info.SessionKey))
	copy(key, c.info.SessionKey)
	return key
}

// SetAgentPublicKey 设置 Agent 公钥
func (c *AgentConnection) SetAgentPublicKey(publicKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.AgentPublicKey = publicKey
}

// GetAgentPublicKey 获取 Agent 公钥
func (c *AgentConnection) GetAgentPublicKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.AgentPublicKey
}

// SetAgentFingerprint 设置 Agent 公钥指纹
func (c *AgentConnection) SetAgentFingerprint(fingerprint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.AgentFingerprint = fingerprint
}

// GetAgentFingerprint 获取 Agent 公钥指纹
func (c *AgentConnection) GetAgentFingerprint() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.AgentFingerprint
}

// EnableEncryption 启用加密
func (c *AgentConnection) EnableEncryption() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.info.EncryptionEnabled = true
}

// IsEncryptionEnabled 检查是否启用加密
func (c *AgentConnection) IsEncryptionEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.info.EncryptionEnabled
}

// WriteEncryptedJSON 发送加密的 JSON 消息
func (c *AgentConnection) WriteEncryptedJSON(v interface{}) error {
	// 检查是否启用加密
	if !c.IsEncryptionEnabled() {
		// 未启用加密，使用普通方式发送
		return c.WriteJSON(v)
	}

	// 获取会话密钥
	sessionKey := c.GetSessionKey()
	if sessionKey == nil {
		return ErrConnectionClosed
	}

	// 序列化 JSON
	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// 使用 AES-GCM 加密消息
	encryptedData, err := encryptMessageAES(jsonData, sessionKey)
	if err != nil {
		return err
	}

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

	// 直接发送二进制消息（与 Agent 端保持一致）
	return c.conn.WriteMessage(websocket.BinaryMessage, encryptedData)
}

// ReadEncryptedMessage 读取加密消息
func (c *AgentConnection) ReadEncryptedMessage() ([]byte, error) {
	// 检查是否启用加密
	if !c.IsEncryptionEnabled() {
		// 未启用加密，使用普通方式读取
		_, message, err := c.ReadMessage()
		return message, err
	}

	// 获取会话密钥
	sessionKey := c.GetSessionKey()
	if sessionKey == nil {
		return nil, ErrConnectionClosed
	}

	// 设置读取超时
	if c.config != nil && c.config.ReadTimeout > 0 {
		deadline := time.Now().Add(c.config.ReadTimeout)
		if err := c.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
	}

	// 读取消息
	messageType, message, err := c.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	// 如果是二进制消息，直接解密
	if messageType == websocket.BinaryMessage {
		// 使用 AES-GCM 解密消息
		decryptedData, err := decryptMessageAES(message, sessionKey)
		if err != nil {
			return nil, err
		}
		return decryptedData, nil
	}

	// 如果是文本消息，可能是明文消息（向后兼容）
	return message, nil
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

// encryptMessageAES 使用 AES-GCM 加密消息（内部函数，避免导入循环）
func encryptMessageAES(message []byte, key []byte) ([]byte, error) {
	// 验证密钥长度（AES-256 需要 32 字节）
	if len(key) != 32 {
		return nil, errors.New("密钥长度必须是 32 字节（AES-256）")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 生成 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}

	// 加密消息
	ciphertext := gcm.Seal(nonce, nonce, message, nil)

	return ciphertext, nil
}

// decryptMessageAES 使用 AES-GCM 解密消息（内部函数，避免导入循环）
func decryptMessageAES(encryptedMessage []byte, key []byte) ([]byte, error) {
	// 验证密钥长度（AES-256 需要 32 字节）
	if len(key) != 32 {
		return nil, errors.New("密钥长度必须是 32 字节（AES-256）")
	}

	// 验证加密消息长度
	if len(encryptedMessage) < 12 {
		return nil, errors.New("加密消息长度不足")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建 AES cipher 失败: %w", err)
	}

	// 创建 GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建 GCM 失败: %w", err)
	}

	// 提取 nonce 和密文
	nonceSize := gcm.NonceSize()
	nonce := encryptedMessage[:nonceSize]
	ciphertext := encryptedMessage[nonceSize:]

	// 解密消息
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}

	return plaintext, nil
}

package websocket

import (
	"net"
	nethttp "net/http"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"github.com/gorilla/websocket"
)

// Upgrader WebSocket 升级器
type Upgrader struct {
	upgrader websocket.Upgrader
	config   *Config
}

// NewUpgrader 创建 WebSocket 升级器
func NewUpgrader(config *Config) *Upgrader {
	if config == nil {
		config = DefaultConfig()
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *nethttp.Request) bool {
			if config.CheckOrigin != nil {
				return config.CheckOrigin(r)
			}
			// 默认允许所有来源（生产环境应该更严格检查）
			return true
		},
	}

	return &Upgrader{
		upgrader: upgrader,
		config:   config,
	}
}

// Upgrade 升级 HTTP 连接为 WebSocket
func (u *Upgrader) Upgrade(w nethttp.ResponseWriter, r *nethttp.Request, responseHeader nethttp.Header) (*websocket.Conn, error) {
	return u.upgrader.Upgrade(w, r, responseHeader)
}

// ExtractIPFromAddr 从 net.Addr 中提取IP地址
func (u *Upgrader) ExtractIPFromAddr(addr net.Addr) string {
	if addr == nil {
		return "unknown"
	}
	return u.ExtractIPFromAddrString(addr.String())
}

// ExtractIPFromAddrString 从地址字符串中提取IP地址
func (u *Upgrader) ExtractIPFromAddrString(addrStr string) string {
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

// GetClientIPFromConn 获取客户端真实IP地址
func (u *Upgrader) GetClientIPFromConn(conn *websocket.Conn, ctx http.Context) string {
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
		return u.ExtractIPFromAddr(conn.RemoteAddr())
	}

	return ctx.Request().Ip()
}


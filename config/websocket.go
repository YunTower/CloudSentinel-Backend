package config

import (
	"goravel/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("websocket", map[string]any{
		// 生产环境下额外放行的 WebSocket Origin（host:port，英文逗号分隔）。
		// 默认为空：仅允许与 http.url 同源的连接。
		"allowed_origins": config.Env("WEBSOCKET_ALLOWED_ORIGINS", ""),
	})
}

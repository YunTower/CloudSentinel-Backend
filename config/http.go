package config

import (
	"github.com/gin-gonic/gin/render"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/gin"
	ginfacades "github.com/goravel/gin/facades"
	"goravel/app/facades"
)

func init() {
	config := facades.Config()
	config.Add("http", map[string]any{
		// HTTP Driver
		"default": "gin",
		// HTTP Drivers
		"drivers": map[string]any{
			"gin": map[string]any{
				// Optional, default is 4096 KB
				"body_limit":   4096,
				"header_limit": 4096,
				"route": func() (route.Route, error) {
					return ginfacades.Route("gin"), nil
				},
				// Optional, default is http/template
				"template": func() (render.HTMLRender, error) {
					return gin.DefaultTemplate()
				},
			},
		},
		// HTTP URL
		"url": config.Env("APP_URL", "http://localhost"),
		// HTTP Host
		"host": config.Env("APP_HOST", "127.0.0.1"),
		// HTTP Port
		"port": config.Env("APP_PORT", "3000"),
		// HTTP Timeout, default is 3 seconds
		"request_timeout": 3,
		// HTTPS Configuration
		"tls": map[string]any{
			// HTTPS Host
			"host": config.Env("APP_HOST", "127.0.0.1"),
			// HTTPS Port
			"port": config.Env("APP_PORT", "3000"),
			// SSL Certificate, you can put the certificate in /public folder
			"ssl": map[string]any{
				// 面板 HTTPS/WSS 证书与私钥（PEM）。两者同时配置后，面板以 HTTPS 监听，
				// Agent 必须通过 wss:// 连接（除非显式开启 allow_insecure_transport）。
				// 自签证书可用 scripts/certs/generate-certs.sh 生成。
				"cert": config.Env("TLS_CERT_FILE", ""),
				"key":  config.Env("TLS_KEY_FILE", ""),
			},
		},
		// HTTP Client Configuration
		"default_client": config.Env("HTTP_CLIENT_DEFAULT", "default"),
		"clients": map[string]any{
			"default": map[string]any{
				"base_url":                config.Env("HTTP_CLIENT_BASE_URL", ""),
				"timeout":                 config.Env("HTTP_CLIENT_TIMEOUT", "30s"),
				"max_idle_conns":          config.Env("HTTP_CLIENT_MAX_IDLE_CONNS", 100),
				"max_idle_conns_per_host": config.Env("HTTP_CLIENT_MAX_IDLE_CONNS_PER_HOST", 2),
				"max_conns_per_host":      config.Env("HTTP_CLIENT_MAX_CONN_PER_HOST", 0),
				"idle_conn_timeout":       config.Env("HTTP_CLIENT_IDLE_CONN_TIMEOUT", "90s"),
			},
			"github": map[string]any{
				"base_url":                config.Env("HTTP_CLIENT_GITHUB_BASE_URL", "https://api.github.com"),
				"timeout":                 config.Env("HTTP_CLIENT_GITHUB_TIMEOUT", "30s"),
				"max_idle_conns":          20,
				"max_idle_conns_per_host": 10,
				"idle_conn_timeout":       "90s",
			},
			"download": map[string]any{
				"timeout":                 config.Env("HTTP_CLIENT_DOWNLOAD_TIMEOUT", "10m"),
				"max_idle_conns":          10,
				"max_idle_conns_per_host": 5,
				"idle_conn_timeout":       "90s",
			},
		},
	})
}

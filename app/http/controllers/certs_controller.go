package controllers

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type CertsController struct{}

func NewCertsController() *CertsController {
	return &CertsController{}
}

// GetCA 返回面板自签 CA 证书（PEM 文本），供 Agent 安装时自动获取并信任。
// CA 公钥本身无保密需求（任何客户端都需要持有它来校验面板），公开返回不构成泄露；
// 面板未配置证书（纯 HTTP 模式）时返回 404。
func (c *CertsController) GetCA(ctx http.Context) http.Response {
	certFile := facades.Config().GetString("http.tls.ssl.cert")
	if certFile == "" {
		return ctx.Response().Status(http.StatusNotFound).String("not configured")
	}

	caFile := filepath.Join(filepath.Dir(certFile), "ca.crt")
	pemBytes, err := os.ReadFile(caFile)
	if err != nil {
		return ctx.Response().Status(http.StatusNotFound).String("ca not found")
	}

	text := strings.TrimSpace(string(pemBytes))
	if !strings.Contains(text, "BEGIN CERTIFICATE") {
		return ctx.Response().Status(http.StatusInternalServerError).String("invalid ca file")
	}
	return ctx.Response().Header("Content-Type", "application/x-pem-file").Status(http.StatusOK).String(text)
}

// Bootstrap 是 Agent 首次启动的统一引导接口：一次请求返回全部引导配置，
// Agent 侧无需逐项请求（未来新增引导项在此扩展即可）。当前包含：
//
//	ca           面板自签 CA（PEM 文本）；面板未启用 TLS 时为空字符串
//	panel_version 面板版本号（agent 可据此判断兼容性）
//	server_time  面板当前 Unix 时间（秒），agent 可据此校准本地时钟
//	wss_enabled  面板是否以 HTTPS/WSS 监听（强制 wss 时 agent 不得回退 ws://）
func (c *CertsController) Bootstrap(ctx http.Context) http.Response {
	data := map[string]any{
		"ca":            "",
		"panel_version": facades.Config().GetString("app.version", "0.0.1-release"),
		"server_time":   time.Now().Unix(),
		"wss_enabled":   false,
	}

	certFile := facades.Config().GetString("http.tls.ssl.cert")
	if certFile != "" {
		caFile := filepath.Join(filepath.Dir(certFile), "ca.crt")
		if pemBytes, err := os.ReadFile(caFile); err == nil {
			text := strings.TrimSpace(string(pemBytes))
			if strings.Contains(text, "BEGIN CERTIFICATE") {
				data["ca"] = text
				data["wss_enabled"] = true
			}
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data":    data,
	})
}

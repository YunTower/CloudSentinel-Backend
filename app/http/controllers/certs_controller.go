package controllers

import (
	"os"
	"path/filepath"
	"strings"

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

// Bootstrap 是 Agent 首次启动的统一引导接口：一次请求返回全部引导配置。
// 当前返回面板自签 CA（未启用 TLS 时为空字符串）；后续需要下发其他引导
// 配置（版本约束、时间同步等）时在此扩展，Agent 侧无需多次请求。
func (c *CertsController) Bootstrap(ctx http.Context) http.Response {
	data := map[string]any{
		"ca": "",
	}

	certFile := facades.Config().GetString("http.tls.ssl.cert")
	if certFile != "" {
		caFile := filepath.Join(filepath.Dir(certFile), "ca.crt")
		if pemBytes, err := os.ReadFile(caFile); err == nil {
			text := strings.TrimSpace(string(pemBytes))
			if strings.Contains(text, "BEGIN CERTIFICATE") {
				data["ca"] = text
			}
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data":    data,
	})
}

package controllers

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/cryptoutil"
	"goravel/app/facades"
	"goravel/app/services/websocket/panelkey"
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

// Bootstrap 是 Agent 首次启动的统一引导接口（公开、未认证）。
//
// 安全边界：CA 虽然是公开材料，但首次下载通道尚未可信。面板用与 Agent 预置
// 指纹对应的 RSA 私钥签名 CA 文本，Agent 验签成功才会持久化为信任锚。
func (c *CertsController) Bootstrap(ctx http.Context) http.Response {
	ca := ""
	panelPublicKey := ""
	panelFingerprint := ""
	caSignature := ""
	certFile := facades.Config().GetString("http.tls.ssl.cert")
	if certFile != "" {
		caFile := filepath.Join(filepath.Dir(certFile), "ca.crt")
		if pemBytes, err := os.ReadFile(caFile); err == nil {
			text := strings.TrimSpace(string(pemBytes))
			if strings.Contains(text, "BEGIN CERTIFICATE") {
				ca = text
			}
		}
	}
	if ca != "" {
		privateKey, publicKey, err := panelkey.GetOrGenerate()
		if err == nil {
			fingerprint, fpErr := cryptoutil.GetPublicKeyFingerprint(publicKey)
			signature, sigErr := cryptoutil.SignData([]byte(bootstrapCASignatureContext+ca), privateKey)
			if fpErr == nil && sigErr == nil {
				panelPublicKey = publicKey
				panelFingerprint = fingerprint
				caSignature = base64.StdEncoding.EncodeToString(signature)
			} else {
				facades.Log().Errorf("签署 Agent 引导 CA 失败: fingerprint=%v signature=%v", fpErr, sigErr)
				ca = ""
			}
		} else {
			facades.Log().Errorf("读取面板密钥以签署 Agent 引导 CA 失败: %v", err)
			ca = ""
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"ca":                ca, // 面板未启用 TLS 或签名不可用时为空字符串
			"panel_public_key":  panelPublicKey,
			"panel_fingerprint": panelFingerprint,
			"ca_signature":      caSignature,
		},
	})
}

const bootstrapCASignatureContext = "CloudSentinel bootstrap CA v1:"

package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goravel/app/facades"
	"goravel/app/utils/envfile"
)

// 面板 HTTPS/WSS 自签证书管理（P2-03）。
// 证书内建到面板本体，全部使用 Go 标准库 crypto/x509 实现，无需安装 openssl：
//   - 首次启动（生产环境）自动生成并写入 .env（EnsureTLSCertificates）
//   - cert:generate / cert:renew / cert:info 命令复用同一实现
// Agent 侧：wss:// 默认启用；首次启动时若未配置 tls_ca_file，会从面板
// GET /api/certs/ca 自动获取 CA 后建立连接（详见 agent/internal/agent）。

// TLS 证书文件约定（与 cert:generate 命令共用）
const (
	TLSCertDirDefault = "storage/certs"
	TLSCACertFile     = "ca.crt"
	TLSCAKeyFile      = "ca.key"
	TLSPanelCertFile  = "panel.crt"
	TLSPanelKeyFile   = "panel.key"
	DefaultCertDays   = 3650
)

// CertDir 返回证书输出目录（默认 storage/certs 的绝对路径）。
func CertDir() string {
	abs, err := filepath.Abs(TLSCertDirDefault)
	if err != nil {
		return TLSCertDirDefault
	}
	return abs
}

// EnsureTLSCertificates 面板首次启动时自动生成自签证书（P2-03）。
//
// 仅在以下情况自动生成：尚未配置 TLS_CERT_FILE（或文件不存在）且为生产环境——
// 安装脚本生成的 .env 中 APP_ENV=production，因此"安装完成、首次启动"即自动签发；
// 本地开发（local/development）不自动启用 HTTPS，避免破坏开发体验。
// 已配置证书时不做任何改动（管理员可通过 cert:renew 续期）。
func EnsureTLSCertificates() {
	certFile := facades.Config().GetString("http.tls.ssl.cert")
	keyFile := facades.Config().GetString("http.tls.ssl.key")
	if certFile != "" && keyFile != "" && fileExists(certFile) && fileExists(keyFile) {
		return
	}
	if env := facades.Config().GetString("app.env"); env != "" && env != "production" {
		return
	}

	outDir := CertDir()
	if err := os.MkdirAll(outDir, 0700); err != nil {
		facades.Log().Warningf("自动生成 TLS 证书失败（创建目录）: %v", err)
		return
	}

	facades.Log().Infof("首次启动：自动生成面板 HTTPS/WSS 自签证书到 %s", outDir)
	if err := generateCAIfMissing(outDir, DefaultCertDays); err != nil {
		facades.Log().Warningf("自动生成 TLS 证书失败（CA）: %v", err)
		return
	}
	if err := issuePanelCert(outDir, DefaultCertDays, nil, nil); err != nil {
		facades.Log().Warningf("自动生成 TLS 证书失败（面板证书）: %v", err)
		return
	}
	updateEnvTLS(outDir)
}

// GenerateCertificates 签发 CA（存在则复用）与面板证书，供 cert:generate 命令调用。
func GenerateCertificates(outDir string, days int, domains, ips []string) error {
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return err
	}
	if err := generateCAIfMissing(outDir, days); err != nil {
		return err
	}
	return issuePanelCert(outDir, days, domains, ips)
}

// RenewPanelCert 复用 CA 续期面板证书；domains/ips 为空时沿用现有证书 SAN。
func RenewPanelCert(outDir string, days int, domains, ips []string) error {
	caPath := filepath.Join(outDir, TLSCACertFile)
	if !fileExists(caPath) {
		return fmt.Errorf("未找到 CA（%s），请先执行 cert:generate", caPath)
	}
	panelPath := filepath.Join(outDir, TLSPanelCertFile)
	if !fileExists(panelPath) {
		return fmt.Errorf("未找到现有面板证书（%s），请先执行 cert:generate", panelPath)
	}
	if len(domains) == 0 && len(ips) == 0 {
		if err := ReadExistingSAN(panelPath, &domains, &ips); err != nil {
			return err
		}
	}
	return issuePanelCert(outDir, days, domains, ips)
}

// CertSummary 返回证书可读摘要（Subject/Issuer/有效期/SAN）。
func CertSummary(path string) (string, error) {
	der, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(der)
	if block == nil {
		return "", fmt.Errorf("不是有效的 PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Subject: %s\n", cert.Subject.CommonName)
	fmt.Fprintf(&b, "Issuer: %s\n", cert.Issuer.CommonName)
	fmt.Fprintf(&b, "NotBefore: %s\n", cert.NotBefore.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "NotAfter: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05"))
	if len(cert.DNSNames) > 0 || len(cert.IPAddresses) > 0 {
		san := append([]string{}, cert.DNSNames...)
		for _, ip := range cert.IPAddresses {
			san = append(san, ip.String())
		}
		fmt.Fprintf(&b, "SAN: %s\n", strings.Join(san, ", "))
	}
	return b.String(), nil
}

// ReadExistingSAN 读取现有面板证书的 SAN（renew 沿用）。
func ReadExistingSAN(panelPath string, domains *[]string, ips *[]string) error {
	der, err := os.ReadFile(panelPath)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(der)
	if block == nil {
		return fmt.Errorf("不是有效的 PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	*domains = cert.DNSNames
	*ips = make([]string, 0, len(cert.IPAddresses))
	for _, ip := range cert.IPAddresses {
		*ips = append(*ips, ip.String())
	}
	return nil
}

// UpdateEnvTLS 将证书路径写入 .env（TLS_CERT_FILE / TLS_KEY_FILE），
// 面板重启后即以 HTTPS/WSS 监听；返回是否实际写入。
func UpdateEnvTLS(outDir string) bool {
	certPath := filepath.Join(outDir, TLSPanelCertFile)
	keyPath := filepath.Join(outDir, TLSPanelKeyFile)
	changed := updateEnvValue("TLS_CERT_FILE", certPath)
	if updateEnvValue("TLS_KEY_FILE", keyPath) {
		changed = true
	}
	return changed
}

// CheckTLSCertificateExpiry 检查面板 TLS 证书剩余有效期并输出告警日志。
//
// 面板以 HTTPS/WSS 监听（TLS_CERT_FILE/TLS_KEY_FILE 已配置）时，证书过期会导致
// Agent 无法连接；此处按剩余时间分档告警，提醒执行 cert:renew。
// 未配置证书（纯 HTTP 模式）时不做检查；证书缺失/损坏时仅警告，不阻止启动。
func CheckTLSCertificateExpiry() {
	certFile := facades.Config().GetString("http.tls.ssl.cert")
	if certFile == "" || !fileExists(certFile) {
		return
	}

	notAfter, err := parseCertExpiry(certFile)
	if err != nil {
		facades.Log().Warningf("TLS 证书过期检查失败: %v", err)
		return
	}

	remaining := time.Until(notAfter)
	renewHint := "请执行 cert:renew 续期"
	switch {
	case remaining <= 0:
		facades.Log().Errorf("TLS 证书已过期（%s），Agent 将无法通过 wss 连接；%s", notAfter.Format("2006-01-02"), renewHint)
	case remaining < 7*24*time.Hour:
		facades.Log().Errorf("TLS 证书将在 %s 过期（剩余 %s），请尽快续期；%s", notAfter.Format("2006-01-02"), remaining.Round(time.Hour), renewHint)
	case remaining < 30*24*time.Hour:
		facades.Log().Warningf("TLS 证书将在 %s 过期（剩余 %s），请计划续期；%s", notAfter.Format("2006-01-02"), remaining.Round(24*time.Hour), renewHint)
	default:
		facades.Log().Infof("TLS 证书有效至 %s（剩余 %s）", notAfter.Format("2006-01-02"), remaining.Round(24*time.Hour))
	}
}

// ---------- 实现 ----------

func generateCAIfMissing(outDir string, days int) error {
	if fileExists(filepath.Join(outDir, TLSCAKeyFile)) && fileExists(filepath.Join(outDir, TLSCACertFile)) {
		return nil // 复用 CA，保持 Agent 信任链
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "CloudSentinel Self-Signed CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 0, days),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(outDir, TLSCACertFile), "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(filepath.Join(outDir, TLSCAKeyFile), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
}

func issuePanelCert(outDir string, days int, domains, ips []string) error {
	caDER, err := os.ReadFile(filepath.Join(outDir, TLSCACertFile))
	if err != nil {
		return err
	}
	caPEM, _ := pem.Decode(caDER)
	if caPEM == nil {
		return fmt.Errorf("CA 证书不是有效的 PEM")
	}
	caCert, err := x509.ParseCertificate(caPEM.Bytes)
	if err != nil {
		return err
	}
	caKeyDER, err := os.ReadFile(filepath.Join(outDir, TLSCAKeyFile))
	if err != nil {
		return err
	}
	caKeyPEM, _ := pem.Decode(caKeyDER)
	if caKeyPEM == nil {
		return fmt.Errorf("CA 私钥不是有效的 PEM")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyPEM.Bytes)
	if err != nil {
		return err
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// 默认 SAN：回环地址；自定义域名/IP 追加（去重，renew 沿用 SAN 时避免重复）
	sanDomains := []string{"localhost"}
	seenDomain := map[string]bool{"localhost": true}
	for _, d := range domains {
		if !seenDomain[d] {
			seenDomain[d] = true
			sanDomains = append(sanDomains, d)
		}
	}
	sanIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	seenIP := map[string]bool{"127.0.0.1": true, "::1": true}
	for _, raw := range ips {
		ip := net.ParseIP(raw)
		if ip == nil {
			return fmt.Errorf("无效的 IP 地址: %s", raw)
		}
		if !seenIP[ip.String()] {
			seenIP[ip.String()] = true
			sanIPs = append(sanIPs, ip)
		}
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "CloudSentinel Panel"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(0, 0, days),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     sanDomains,
		IPAddresses:  sanIPs,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(filepath.Join(outDir, TLSPanelCertFile), "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEM(filepath.Join(outDir, TLSPanelKeyFile), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
}

func writePEM(path, blockType string, der []byte) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0600)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// updateEnvTLS 内部包装：更新 .env 的 TLS 配置并记录日志。
func updateEnvTLS(outDir string) {
	if UpdateEnvTLS(outDir) {
		facades.Log().Infof("已写入 .env 的 TLS_CERT_FILE / TLS_KEY_FILE，本次启动即生效 HTTPS/WSS")
	} else {
		facades.Log().Info("TLS 证书已配置或 .env 不可写，跳过写入")
	}
}

// updateEnvValue 内部包装：更新 .env 的 TLS 配置并记录日志。
// .env 不可读或已是最新值时返回 false（沿用旧行为：静默跳过写入）。
func updateEnvValue(key, value string) bool {
	envPath, err := envfile.Path()
	if err != nil {
		return false
	}
	changed, err := envfile.Update(envPath, key, value)
	return err == nil && changed
}

func parseCertExpiry(certFile string) (time.Time, error) {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return time.Time{}, fmt.Errorf("读取证书失败: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return time.Time{}, fmt.Errorf("证书不是有效的 PEM: %s", certFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("解析证书失败: %w", err)
	}
	return cert.NotAfter, nil
}

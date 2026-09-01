package services

// 服务监测的协议级探测实现（HTTP/TCP/UDP/ICMP/DNS/TLS）。
// 从 service_monitor_service.go 拆出，集中承载各协议的探测与
// 目标解析/SSRF 校验辅助函数，便于独立测试与复用。

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"goravel/app/utils/security"
)

// 检查HTTP服务；checkCert 为 true 时解析叶证书有效期（过期仍可读，便于展示剩余天数）
func checkHTTP(target string, timeoutSec, expectStatus int, expectBody, method, headersJSON, requestBody string, checkCert bool) (certCheckInfo, error) {
	var info certCheckInfo
	tlsConfig := &tls.Config{InsecureSkipVerify: false}
	if checkCert {
		// 允许过期证书完成握手，以便采集 NotAfter；过期判定由下方逻辑负责
		tlsConfig.InsecureSkipVerify = true
	}
	allowPrivate := monitorAllowsPrivateTargets()
	// SSRF 防护：默认禁止探测内网/保留地址；建连时二次校验防 DNS rebinding。
	if err := security.ValidateHostForOutboundRequest(extractTargetHost(target), 2*time.Second, allowPrivate); err != nil {
		return info, err
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			DialContext:     security.SafeDialContext(allowPrivate),
		},
		// 不跟随跨主机重定向，防止通过重定向绕过内网校验
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("停止重定向：次数过多")
			}
			if !allowPrivate {
				if err := security.ValidateHostForOutboundRequest(req.URL.Hostname(), 2*time.Second, false); err != nil {
					return err
				}
			}
			return nil
		},
	}
	method = normalizeRequestMethod(method)
	var body io.Reader
	if requestBody != "" && methodAllowsBody(method) {
		body = strings.NewReader(requestBody)
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return info, err
	}
	headers, err := parseHTTPHeaders(headersJSON)
	if err != nil {
		return info, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if requestBody != "" && methodAllowsBody(method) && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()

	if checkCert {
		leaf := extractLeafCertificate(resp)
		if leaf == nil {
			return info, fmt.Errorf("无法获取 TLS 证书信息")
		}
		info.ExpiresAt = leaf.NotAfter
		info.DaysLeft = int(time.Until(leaf.NotAfter).Hours() / 24)
		now := time.Now()
		if now.Before(leaf.NotBefore) {
			return info, fmt.Errorf("证书尚未生效（生效于 %s）", leaf.NotBefore.Format(time.RFC3339))
		}
		if now.After(leaf.NotAfter) {
			return info, fmt.Errorf("证书已过期（过期于 %s）", leaf.NotAfter.Format(time.RFC3339))
		}
	}

	// 检查状态码
	if expectStatus > 0 {
		if resp.StatusCode != expectStatus {
			return info, fmt.Errorf("期望状态码 %d，实际 %d", expectStatus, resp.StatusCode)
		}
	} else if resp.StatusCode >= 500 {
		return info, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 检查响应体
	if expectBody != "" {
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 限制 1MB 防止内存耗尽
		if err != nil {
			return info, fmt.Errorf("读取响应体失败: %v", err)
		}
		if !strings.Contains(string(respBody), expectBody) {
			return info, fmt.Errorf("响应体不包含期望内容")
		}
	}
	return info, nil
}

func extractLeafCertificate(resp *http.Response) *x509.Certificate {
	if resp == nil || resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return nil
	}
	return resp.TLS.PeerCertificates[0]
}

// extractTargetHost 提取监测目标 URL 的 host 部分，用于出站校验。
func extractTargetHost(target string) string {
	value := strings.TrimSpace(target)
	if strings.Contains(value, "://") {
		if u, err := url.Parse(value); err == nil {
			return u.Hostname()
		}
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return value
}

func normalizeRequestMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "GET"
	}
}

func methodAllowsBody(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func parseHTTPHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	headers := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, fmt.Errorf("http headers must be a JSON object: %w", err)
	}
	for key := range headers {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("http header name cannot be empty")
		}
	}
	return headers, nil
}

// probeTargetHost 从监测目标中提取 host 并执行 SSRF 出站校验。
// 返回校验后的 host，供对应的探测函数使用。返回错误时调用方应放弃探测。
func probeTargetHost(typ, target string, defaultPort int, allowPrivate bool) (string, error) {
	if typ == "http" || typ == "https" {
		// HTTP 探测内部已有完整 SSRF 防护（ValidateHostForOutboundRequest + SafeDialContext），
		// 无需在此重复校验，避免重复 DNS 解析。
		return extractTargetHost(target), nil
	}
	host, _, err := splitMonitorTarget(target, defaultPort)
	if err != nil {
		return "", err
	}
	if err := security.ValidateHostForOutboundRequest(host, 2*time.Second, allowPrivate); err != nil {
		return "", err
	}
	return host, nil
}

// 检查TCP服务
func checkTCP(host string, port int, timeoutSec int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// 检查UDP服务
func checkUDP(host string, port int, timeoutSec int) error {
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func checkICMP(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := []string{"-c", "1", "-W", fmt.Sprintf("%d", timeoutSec), host}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", fmt.Sprintf("%d", timeoutSec*1000), host}
	}
	cmd := exec.CommandContext(ctx, "ping", args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ping timeout")
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ping failed: %s", msg)
	}
	return nil
}

func checkDNS(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("dns lookup failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns lookup returned no records")
	}
	return nil
}

func checkTLS(target string, port int, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, resolvedPort, err := splitMonitorTarget(target, 443)
	if err != nil {
		return err
	}
	if port > 0 {
		resolvedPort = port
	}

	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, resolvedPort), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("tls connection failed: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("tls certificate not found")
	}
	cert := state.PeerCertificates[0]
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("tls certificate not valid before %s", cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("tls certificate expired at %s", cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

func splitMonitorTarget(target string, defaultPort int) (string, int, error) {
	value := strings.TrimSpace(target)
	if value == "" {
		return "", 0, fmt.Errorf("target is empty")
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", 0, err
		}
		host := parsed.Hostname()
		if host == "" {
			return "", 0, fmt.Errorf("target host is empty")
		}
		port := defaultPort
		if parsed.Port() != "" {
			if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil {
				return "", 0, fmt.Errorf("invalid target port")
			}
		}
		return host, port, nil
	}

	host := value
	port := defaultPort
	if h, p, err := net.SplitHostPort(value); err == nil {
		host = h
		if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
			return "", 0, fmt.Errorf("invalid target port")
		}
	}
	if host == "" {
		return "", 0, fmt.Errorf("target host is empty")
	}
	// 拒绝以 "-" 开头的 host：host 会作为 ping 等命令的位置参数，
	// 以 "-" 开头可注入命令选项（如 -f 洪泛）。合法主机名/IP 不会以 - 开头。
	if strings.HasPrefix(host, "-") {
		return "", 0, fmt.Errorf("invalid target host")
	}
	return host, port, nil
}

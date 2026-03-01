package security

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

var blockedHostnames = map[string]struct{}{
	"localhost":                {},
	"localhost.localdomain":    {},
	"metadata":                 {},
	"metadata.google.internal": {},
}

var blockedCIDRs = mustParseCIDRs([]string{
	// IPv4
	"0.0.0.0/8",      // "this" network
	"10.0.0.0/8",     // RFC1918
	"127.0.0.0/8",    // loopback
	"169.254.0.0/16", // link-local (AWS/GCP metadata 等)
	"172.16.0.0/12",  // RFC1918
	"192.168.0.0/16", // RFC1918
	"100.64.0.0/10",  // CGNAT
	"198.18.0.0/15",  // benchmark
	"224.0.0.0/4",    // multicast
	"240.0.0.0/4",    // reserved
	"255.255.255.255/32",

	// IPv6
	"::/128",        // unspecified
	"::1/128",       // loopback
	"fc00::/7",      // ULA
	"fe80::/10",     // link-local
	"ff00::/8",      // multicast
	"2001:db8::/32", // documentation
})

func mustParseCIDRs(cidrs []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, s := range cidrs {
		_, ipnet, err := net.ParseCIDR(s)
		if err != nil {
			panic(err)
		}
		out = append(out, ipnet)
	}
	return out
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	for _, n := range blockedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	// 兜底：覆盖 Go 自带的判定（例如 IsPrivate/IsLoopback 等）。
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	return false
}

// ParseAndValidateWebhookURLForConfig 仅用于“保存配置”的校验：
// - 仅允许 http/https
// - 禁止 userinfo
// - 禁止明显内网/metadata 地址（对域名不做 DNS 解析，避免配置阶段受网络影响）
func ParseAndValidateWebhookURLForConfig(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("webhook URL 不能为空")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("webhook URL 格式错误")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("webhook host 不能为空")
	}
	if u.User != nil {
		return nil, fmt.Errorf("不允许在 URL 中携带账号信息")
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if _, ok := blockedHostnames[host]; ok {
		return nil, fmt.Errorf("禁止访问该 host")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return nil, fmt.Errorf("禁止访问内网/保留地址")
		}
	}

	u.Fragment = ""
	return u, nil
}

// ResolveAndValidateWebhookURLForRequest 用于“真正发起请求”前的校验：
// - 在 ParseAndValidateWebhookURLForConfig 的基础上，对域名做 DNS 解析
// - 若解析结果包含任何内网/保留地址，则拒绝（避免 SSRF）
func ResolveAndValidateWebhookURLForRequest(raw string, timeout time.Duration) (*url.URL, []net.IP, error) {
	u, err := ParseAndValidateWebhookURLForConfig(raw)
	if err != nil {
		return nil, nil, err
	}

	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return u, []net.IP{ip}, nil
	}

	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, nil, fmt.Errorf("webhook host 解析失败")
	}
	if len(addrs) == 0 {
		return nil, nil, fmt.Errorf("webhook host 无可用解析结果")
	}

	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ip := a.IP
		if isBlockedIP(ip) {
			return nil, nil, fmt.Errorf("禁止访问内网/保留地址")
		}
		ips = append(ips, ip)
	}

	return u, ips, nil
}

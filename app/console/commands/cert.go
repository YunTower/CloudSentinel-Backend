package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"goravel/app/services"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

// 面板 HTTPS/WSS 自签证书管理命令（P2-03）。
// 实现复用 services 的 TLS 证书逻辑（Go 标准库，无需 openssl）：
//   cert:generate  首次签发（CA 存在则复用）
//   cert:renew     复用 CA 续期面板证书（未指定 SAN 时沿用现有证书）
//   cert:info      查看 CA / 面板证书状态
//
// 生产环境首次启动时面板会自动生成证书（services.EnsureTLSCertificates），
// 本命令用于手动管理（自定义 SAN、提前续期、查看状态）。

func certFlags() command.Extend {
	return command.Extend{
		Flags: []command.Flag{
			&command.StringFlag{Name: "out-dir", Usage: "证书输出目录（默认 " + services.TLSCertDirDefault + "）"},
			&command.IntFlag{Name: "days", Value: services.DefaultCertDays, Usage: "证书有效期（天）"},
			&command.StringSliceFlag{Name: "domains", Usage: "SAN 域名（可多次，如 --domains=panel.example.com）"},
			&command.StringSliceFlag{Name: "ips", Usage: "SAN IP（可多次，如 --ips=192.168.1.10）"},
			&command.BoolFlag{Name: "no-env", Usage: "不更新 .env（默认会写入 TLS_CERT_FILE/TLS_KEY_FILE）"},
		},
	}
}

func certOutDir(ctx console.Context) string {
	if v := strings.TrimSpace(ctx.Option("out-dir")); v != "" {
		if abs, err := filepath.Abs(v); err == nil {
			return abs
		}
		return v
	}
	return services.CertDir()
}

func certDays(ctx console.Context) int {
	if d := ctx.OptionInt("days"); d > 0 {
		return d
	}
	return services.DefaultCertDays
}

func certSAN(ctx console.Context) ([]string, []string) {
	domains := ctx.OptionSlice("domains")
	ips := ctx.OptionSlice("ips")
	clean := func(items []string) []string {
		out := make([]string, 0, len(items))
		for _, s := range items {
			for _, part := range strings.Split(s, ",") {
				if p := strings.TrimSpace(part); p != "" {
					out = append(out, p)
				}
			}
		}
		return out
	}
	return clean(domains), clean(ips)
}

func certShouldUpdateEnv(ctx console.Context) bool {
	return !ctx.OptionBool("no-env")
}

// GenerateCertCommand 首次签发 CA 与面板证书
type GenerateCertCommand struct{}

func NewGenerateCertCommand() *GenerateCertCommand {
	return &GenerateCertCommand{}
}

func (c *GenerateCertCommand) Signature() string {
	return "cert:generate"
}

func (c *GenerateCertCommand) Description() string {
	return "生成面板 HTTPS/WSS 自签证书（CA + 面板证书）并写入 .env"
}

func (c *GenerateCertCommand) Extend() command.Extend {
	return certFlags()
}

func (c *GenerateCertCommand) Handle(ctx console.Context) error {
	outDir := certOutDir(ctx)
	days := certDays(ctx)
	domains, ips := certSAN(ctx)

	PrintInfo(fmt.Sprintf("正在生成证书到 %s（有效期 %d 天）...", outDir, days))
	if err := services.GenerateCertificates(outDir, days, domains, ips); err != nil {
		PrintError(fmt.Sprintf("生成证书失败: %v", err))
		return err
	}

	if certShouldUpdateEnv(ctx) {
		if services.UpdateEnvTLS(outDir) {
			PrintSuccess(".env 已写入 TLS_CERT_FILE / TLS_KEY_FILE（重启面板后生效 HTTPS/WSS）")
		} else {
			PrintInfo(".env 未找到或已包含相同配置，跳过写入")
		}
	}
	printCertSummary(outDir)
	return nil
}

// RenewCertCommand 复用 CA 续期面板证书（未指定 SAN 时沿用现有证书）
type RenewCertCommand struct{}

func NewRenewCertCommand() *RenewCertCommand {
	return &RenewCertCommand{}
}

func (c *RenewCertCommand) Signature() string {
	return "cert:renew"
}

func (c *RenewCertCommand) Description() string {
	return "复用现有 CA 续期面板证书（Agent 无需重新配置信任）"
}

func (c *RenewCertCommand) Extend() command.Extend {
	return certFlags()
}

func (c *RenewCertCommand) Handle(ctx console.Context) error {
	outDir := certOutDir(ctx)
	days := certDays(ctx)
	domains, ips := certSAN(ctx)
	if len(domains) == 0 && len(ips) == 0 {
		var existingDomains, existingIPs []string
		if err := services.ReadExistingSAN(filepath.Join(outDir, services.TLSPanelCertFile), &existingDomains, &existingIPs); err != nil {
			PrintError(fmt.Sprintf("读取现有证书 SAN 失败: %v", err))
			return err
		}
		domains, ips = existingDomains, existingIPs
		PrintInfo(fmt.Sprintf("沿用现有 SAN: %s", strings.Join(append(domains, ips...), ", ")))
	}

	PrintInfo("复用 CA 续期面板证书（换用全新私钥）...")
	if err := services.RenewPanelCert(outDir, days, domains, ips); err != nil {
		PrintError(fmt.Sprintf("续期失败: %v", err))
		return err
	}

	if certShouldUpdateEnv(ctx) {
		if services.UpdateEnvTLS(outDir) {
			PrintSuccess(".env 已更新 TLS_CERT_FILE / TLS_KEY_FILE（重启面板后生效）")
		}
	}
	printCertSummary(outDir)
	return nil
}

// InfoCertCommand 查看证书状态
type InfoCertCommand struct{}

func NewInfoCertCommand() *InfoCertCommand {
	return &InfoCertCommand{}
}

func (c *InfoCertCommand) Signature() string {
	return "cert:info"
}

func (c *InfoCertCommand) Description() string {
	return "查看面板证书（CA/面板）的签发者、有效期与 SAN"
}

func (c *InfoCertCommand) Extend() command.Extend {
	return command.Extend{
		Flags: []command.Flag{
			&command.StringFlag{Name: "out-dir", Usage: "证书输出目录（默认 " + services.TLSCertDirDefault + "）"},
		},
	}
}

func (c *InfoCertCommand) Handle(ctx console.Context) error {
	outDir := certOutDir(ctx)
	found := false
	for _, name := range []string{services.TLSCACertFile, services.TLSPanelCertFile} {
		path := filepath.Join(outDir, name)
		info, err := services.CertSummary(path)
		if err != nil {
			continue
		}
		found = true
		fmt.Printf("===== %s =====\n%s\n", path, info)
	}
	if !found {
		err := fmt.Errorf("未找到证书（%s/*.crt），请先执行 cert:generate", outDir)
		PrintError(err.Error())
		return err
	}
	return nil
}

func printCertSummary(outDir string) {
	fmt.Println()
	PrintSuccess("证书就绪")
	for _, name := range []string{services.TLSCACertFile, services.TLSPanelCertFile} {
		path := filepath.Join(outDir, name)
		if info, err := services.CertSummary(path); err == nil {
			fmt.Printf("----- %s -----\n%s", path, info)
		}
	}
	fmt.Println()
	PrintInfo("Agent 需信任 CA 并改用 wss://：agent.lock.json 中设置")
	fmt.Printf("    \"tls_ca_file\": %q\n", filepath.Join(outDir, services.TLSCACertFile))
	PrintInfo("Agent 首次启动时若未配置 tls_ca_file，会自动从面板 GET /api/certs/ca 获取")
	PrintInfo("续期：cert:renew（复用 CA，Agent 无需重新配置）")
}

package commands

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"goravel/app/repositories"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/facades"
)

type PanelInfoCommand struct{}

func NewPanelInfoCommand() *PanelInfoCommand {
	return &PanelInfoCommand{}
}

// Signature The name and signature of the console command.
func (c *PanelInfoCommand) Signature() string {
	return "panel:info"
}

// Description The console command description.
func (c *PanelInfoCommand) Description() string {
	return "查看面板信息（会重置管理员账号和密码）"
}

// Extend The console command extend.
func (c *PanelInfoCommand) Extend() command.Extend {
	return command.Extend{}
}

// Handle Execute the console command.
func (c *PanelInfoCommand) Handle(ctx console.Context) error {
	PrintWarning("运行此命令会重置管理员账号和密码！")
	fmt.Println()
	fmt.Print("确定要继续吗？(y/N): ")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		PrintError("读取输入失败")
		return err
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		PrintInfo("已取消")
		return nil
	}

	fmt.Println()

	// 生成随机管理员账号
	PrintInfo("正在生成随机管理员账号...")

	// 生成10位随机用户名
	username, err := GenerateRandomString(10, "alphanumeric")
	if err != nil {
		PrintError(fmt.Sprintf("生成用户名失败: %v", err))
		return err
	}

	// 生成20位随机密码
	password, err := GenerateRandomString(20, "alphanumeric_special")
	if err != nil {
		PrintError(fmt.Sprintf("生成密码失败: %v", err))
		return err
	}

	// 生成密码哈希
	passwordHash, err := facades.Hash().Make(password)
	if err != nil {
		PrintError(fmt.Sprintf("生成密码哈希失败: %v", err))
		return err
	}

	// 更新数据库
	settingRepo := repositories.GetSystemSettingRepository()

	// 更新用户名
	if err := settingRepo.SetValue("admin_username", username); err != nil {
		PrintError(fmt.Sprintf("更新用户名失败: %v", err))
		return err
	}

	// 更新密码哈希
	if err := settingRepo.SetValue("admin_password_hash", passwordHash); err != nil {
		PrintError(fmt.Sprintf("更新密码哈希失败: %v", err))
		return err
	}

	// 获取面板地址和端口
	appHost := facades.Config().GetString("http.host", "0.0.0.0")
	appPort := facades.Config().GetString("http.port", "3000")

	// 获取所有可用IP地址
	allIPs := getAllIPs()

	// 输出结果
	PrintSuccess("管理员账号已重置")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("  %s管理员账号：%s\n", ColorCyan, ColorReset)
	fmt.Printf("  %s用户名:%s     %s%s%s\n", ColorCyan, ColorReset, ColorGreen, username, ColorReset)
	fmt.Printf("  %s密码:%s       %s%s%s\n", ColorCyan, ColorReset, ColorGreen, password, ColorReset)
	fmt.Println()
	fmt.Printf("  %s访问地址：%s\n", ColorCyan, ColorReset)

	// 显示所有IP地址的访问链接
	for _, ip := range allIPs {
		if ip != "" {
			fmt.Printf("  %shttp://%s:%s%s\n", BOLD, ip, appPort, ColorReset)
		}
	}

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	PrintWarning("请妥善保管管理员账号和密码，建议立即登录并修改密码")

	return nil
}

// getAllIPs 获取所有可用的IP地址
func getAllIPs() []string {
	var ips []string

	// 获取所有网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		// 如果获取失败，返回 localhost
		return []string{"127.0.0.1"}
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// 获取接口的所有地址
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// 只处理 IPv4 地址，排除回环地址
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				ipStr := ip.String()
				// 去重
				found := false
				for _, existingIP := range ips {
					if existingIP == ipStr {
						found = true
						break
					}
				}
				if !found {
					ips = append(ips, ipStr)
				}
			}
		}
	}

	// 如果没有找到任何IP，返回 localhost
	if len(ips) == 0 {
		ips = append(ips, "127.0.0.1")
	}

	return ips
}

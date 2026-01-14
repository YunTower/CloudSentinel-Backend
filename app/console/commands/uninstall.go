package commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type UninstallCommand struct{}

func NewUninstallCommand() *UninstallCommand {
	return &UninstallCommand{}
}

// Signature The name and signature of the console command.
func (c *UninstallCommand) Signature() string {
	return "uninstall"
}

// Description The console command description.
func (c *UninstallCommand) Description() string {
	return "卸载 CloudSentinel Dashboard（删除 systemd 配置、全局命令、数据库、日志等）"
}

// Extend The console command extend.
func (c *UninstallCommand) Extend() command.Extend {
	return command.Extend{}
}

// Handle Execute the console command.
func (c *UninstallCommand) Handle(ctx console.Context) error {
	PrintWarning("此操作将删除 CloudSentinel Dashboard 的以下内容：")
	fmt.Println()
	fmt.Println("  - systemd 服务配置")
	fmt.Println("  - 全局命令")
	fmt.Println("  - 数据库文件")
	fmt.Println("  - 日志文件")
	fmt.Println()
	PrintWarning("此操作不可逆！")
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

	// 获取可执行文件路径和安装目录
	exePath, err := os.Executable()
	if err != nil {
		PrintError("获取可执行文件路径失败")
		return err
	}
	installDir := filepath.Dir(exePath)

	// 1. 停止服务
	if err := c.stopService(); err != nil {
		PrintWarning(fmt.Sprintf("停止服务时出现警告: %v", err))
	}

	// 2. 删除 systemd 配置
	if err := c.uninstallSystemdService(); err != nil {
		PrintWarning(fmt.Sprintf("删除 systemd 配置时出现警告: %v", err))
	}

	// 3. 删除全局命令
	if err := c.removeGlobalCommand(); err != nil {
		PrintWarning(fmt.Sprintf("删除全局命令时出现警告: %v", err))
	}

	// 4. 删除数据库文件
	if err := c.removeDatabaseFile(installDir); err != nil {
		PrintWarning(fmt.Sprintf("删除数据库文件时出现警告: %v", err))
	}

	// 5. 删除日志文件
	if err := c.removeLogFiles(installDir); err != nil {
		PrintWarning(fmt.Sprintf("删除日志文件时出现警告: %v", err))
	}

	// 6. 询问是否删除二进制文件
	fmt.Println()
	fmt.Print("是否删除二进制文件？(y/N): ")
	answer, _ = reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "y" || answer == "yes" {
		if err := c.removeBinaryFile(exePath); err != nil {
			PrintWarning(fmt.Sprintf("删除二进制文件时出现警告: %v", err))
		}
	}

	// 7. 询问是否删除安装目录
	fmt.Println()
	fmt.Print("是否删除整个安装目录？(y/N): ")
	answer, _ = reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "y" || answer == "yes" {
		if err := c.removeInstallDirectory(installDir); err != nil {
			PrintWarning(fmt.Sprintf("删除安装目录时出现警告: %v", err))
		}
	}

	// 8. 询问是否删除用户（仅 Linux，需要 root）
	if runtime.GOOS == "linux" && os.Geteuid() == 0 {
		fmt.Println()
		fmt.Print("是否删除 cloudsentinel 用户？(y/N): ")
		answer, _ = reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "y" || answer == "yes" {
			if err := c.removeUser(); err != nil {
				PrintWarning(fmt.Sprintf("删除用户时出现警告: %v", err))
			}
		}
	}

	fmt.Println()
	PrintSuccess("卸载完成")

	return nil
}

// stopService 停止服务
func (c *UninstallCommand) stopService() error {
	// 检查是否为 Linux 系统
	if runtime.GOOS != "linux" {
		return nil
	}

	// 检查 systemd 服务是否存在
	if !ServiceExists() {
		PrintInfo("systemd 服务未安装，跳过停止服务")
		return nil
	}

	// 检查服务是否运行中
	active, err := IsServiceActive()
	if err != nil {
		return fmt.Errorf("检查服务状态失败: %w", err)
	}

	if !active {
		PrintInfo("服务未运行，跳过停止服务")
		return nil
	}

	PrintInfo("正在停止服务...")
	if err := StopService(); err != nil {
		return err
	}

	PrintSuccess("服务已停止")
	return nil
}

// uninstallSystemdService 卸载 systemd 服务
func (c *UninstallCommand) uninstallSystemdService() error {
	// 检查是否为 Linux 系统
	if runtime.GOOS != "linux" {
		return nil
	}

	// 检查是否为 root 用户
	if os.Geteuid() != 0 {
		PrintWarning("删除 systemd 服务需要 root 权限，跳过")
		return nil
	}

	// 检查服务是否存在
	if !ServiceExists() {
		PrintInfo("systemd 服务未安装，跳过删除")
		return nil
	}

	PrintInfo("正在卸载 systemd 服务...")

	// 禁用服务
	if err := DisableService(); err != nil {
		PrintWarning(fmt.Sprintf("禁用服务时出现警告: %v", err))
	}

	// 删除服务文件
	if err := UninstallService(); err != nil {
		return err
	}

	// 重新加载 systemd daemon
	if err := ReloadDaemon(); err != nil {
		return err
	}

	PrintSuccess("systemd 服务已卸载")
	return nil
}

// removeGlobalCommand 删除全局命令
func (c *UninstallCommand) removeGlobalCommand() error {
	globalCmdPath := GetGlobalCommandPath()
	if globalCmdPath == "" {
		PrintWarning("无法确定全局命令路径，跳过删除")
		return nil
	}

	// 检查文件是否存在
	if _, err := os.Stat(globalCmdPath); os.IsNotExist(err) {
		PrintInfo("全局命令不存在，跳过删除")
		return nil
	}

	PrintInfo(fmt.Sprintf("正在删除全局命令: %s", globalCmdPath))
	if err := os.Remove(globalCmdPath); err != nil {
		return fmt.Errorf("删除全局命令失败: %w", err)
	}

	PrintSuccess("全局命令已删除")
	return nil
}

// removeDatabaseFile 删除数据库文件
func (c *UninstallCommand) removeDatabaseFile(installDir string) error {
	// 读取 .env 文件获取数据库配置
	envFile := filepath.Join(installDir, ".env")
	dbName := "forge" // 默认数据库名

	if _, err := os.Stat(envFile); err == nil {
		// 尝试读取 DB_DATABASE 配置
		if dbValue, err := ReadEnvValue(envFile, "DB_DATABASE"); err == nil {
			dbName = dbValue
		}
	}

	// 构建数据库文件路径（SQLite）
	dbFile := filepath.Join(installDir, dbName)

	// 检查文件是否存在
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		PrintInfo(fmt.Sprintf("数据库文件不存在: %s，跳过删除", dbFile))
		return nil
	}

	PrintInfo(fmt.Sprintf("正在删除数据库文件: %s", dbFile))
	if err := os.Remove(dbFile); err != nil {
		return fmt.Errorf("删除数据库文件失败: %w", err)
	}

	PrintSuccess("数据库文件已删除")
	return nil
}

// removeLogFiles 删除日志文件
func (c *UninstallCommand) removeLogFiles(installDir string) error {
	logsDir := filepath.Join(installDir, "storage", "logs")

	// 检查目录是否存在
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		PrintInfo("日志目录不存在，跳过删除")
		return nil
	}

	PrintInfo(fmt.Sprintf("正在删除日志文件: %s", logsDir))

	// 读取目录内容
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return fmt.Errorf("读取日志目录失败: %w", err)
	}

	removedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(logsDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				PrintWarning(fmt.Sprintf("删除日志文件失败: %s, %v", filePath, err))
				continue
			}
			removedCount++
		}
	}

	if removedCount > 0 {
		PrintSuccess(fmt.Sprintf("已删除 %d 个日志文件", removedCount))
	} else {
		PrintInfo("没有找到日志文件")
	}

	return nil
}

// removeBinaryFile 删除二进制文件
func (c *UninstallCommand) removeBinaryFile(exePath string) error {
	PrintInfo(fmt.Sprintf("正在删除二进制文件: %s", exePath))

	// 检查文件是否存在
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		PrintInfo("二进制文件不存在，跳过删除")
		return nil
	}

	if err := os.Remove(exePath); err != nil {
		return fmt.Errorf("删除二进制文件失败: %w", err)
	}

	PrintSuccess("二进制文件已删除")
	return nil
}

// removeInstallDirectory 删除安装目录
func (c *UninstallCommand) removeInstallDirectory(installDir string) error {
	PrintInfo(fmt.Sprintf("正在删除安装目录: %s", installDir))

	// 检查目录是否存在
	if _, err := os.Stat(installDir); os.IsNotExist(err) {
		PrintInfo("安装目录不存在，跳过删除")
		return nil
	}

	if err := os.RemoveAll(installDir); err != nil {
		return fmt.Errorf("删除安装目录失败: %w", err)
	}

	PrintSuccess("安装目录已删除")
	return nil
}

// removeUser 删除用户
func (c *UninstallCommand) removeUser() error {
	// 检查是否为 root 用户
	if os.Geteuid() != 0 {
		PrintWarning("删除用户需要 root 权限，跳过")
		return nil
	}

	// 检查用户是否存在
	cmd := exec.Command("id", "cloudsentinel")
	if err := cmd.Run(); err != nil {
		PrintInfo("cloudsentinel 用户不存在，跳过删除")
		return nil
	}

	PrintInfo("正在删除 cloudsentinel 用户...")

	// 删除用户
	cmd = exec.Command("userdel", "cloudsentinel")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("删除用户失败: %w", err)
	}

	PrintSuccess("用户已删除")
	return nil
}

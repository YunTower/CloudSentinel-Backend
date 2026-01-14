package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GetServiceFilePath 获取 systemd 服务文件路径
func GetServiceFilePath() string {
	return "/etc/systemd/system/cloudsentinel.service"
}

// ServiceExists 检查 systemd 服务文件是否存在
func ServiceExists() bool {
	servicePath := GetServiceFilePath()
	_, err := os.Stat(servicePath)
	return err == nil
}

// ReloadDaemon 重新加载 systemd daemon
func ReloadDaemon() error {
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重新加载systemd daemon失败: %w", err)
	}
	return nil
}

// EnableService 启用 systemd 服务（开机自启）
func EnableService() error {
	cmd := exec.Command("systemctl", "enable", "cloudsentinel.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}
	return nil
}

// DisableService 禁用 systemd 服务
func DisableService() error {
	cmd := exec.Command("systemctl", "disable", "cloudsentinel.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("禁用服务失败: %w", err)
	}
	return nil
}

// StartService 启动 systemd 服务
func StartService() error {
	cmd := exec.Command("systemctl", "start", "cloudsentinel.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动服务失败: %w, 输出: %s", err, string(output))
	}
	return nil
}

// StopService 停止 systemd 服务
func StopService() error {
	cmd := exec.Command("systemctl", "stop", "cloudsentinel.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}
	return nil
}

// RestartService 重启 systemd 服务
func RestartService() error {
	cmd := exec.Command("systemctl", "restart", "cloudsentinel.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}
	return nil
}

// GetServiceStatus 获取服务状态
func GetServiceStatus() (string, error) {
	cmd := exec.Command("systemctl", "is-active", "cloudsentinel.service")
	output, err := cmd.Output()
	if err != nil {
		return "inactive", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// IsServiceActive 检查服务是否处于活动状态
func IsServiceActive() (bool, error) {
	status, err := GetServiceStatus()
	if err != nil {
		return false, err
	}
	return status == "active", nil
}

// UninstallService 卸载 systemd 服务
func UninstallService() error {
	servicePath := GetServiceFilePath()
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除service文件失败: %w", err)
	}
	return nil
}

// GetGlobalCommandPath 获取全局命令路径
func GetGlobalCommandPath() string {
	if os.Geteuid() == 0 {
		// root 用户：使用 /usr/local/bin
		return "/usr/local/bin/cloudsentinel"
	}
	// 非 root 用户：使用 ~/.local/bin
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".local", "bin", "cloudsentinel")
}

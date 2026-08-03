package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"goravel/app/facades"
)

const systemctlTimeout = 30 * time.Second

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
	return runSystemctl("重新加载 systemd daemon", "daemon-reload")
}

// EnableService 启用 systemd 服务（开机自启）
func EnableService() error {
	return runSystemctl("启用服务", "enable", "cloudsentinel.service")
}

// DisableService 禁用 systemd 服务
func DisableService() error {
	return runSystemctl("禁用服务", "disable", "cloudsentinel.service")
}

// StartService 启动 systemd 服务
func StartService() error {
	return runSystemctl("启动服务", "start", "cloudsentinel.service")
}

// StopService 停止 systemd 服务
func StopService() error {
	return runSystemctl("停止服务", "stop", "cloudsentinel.service")
}

// RestartService 重启 systemd 服务
func RestartService() error {
	return runSystemctl("重启服务", "restart", "cloudsentinel.service")
}

// GetServiceStatus 获取服务状态
func GetServiceStatus() (string, error) {
	result := facades.Process().Timeout(systemctlTimeout).Quietly().Run("systemctl", "is-active", "cloudsentinel.service")
	if result.Failed() {
		return "inactive", nil
	}
	return strings.TrimSpace(result.Output()), nil
}

func runSystemctl(operation string, args ...string) error {
	result := facades.Process().Timeout(systemctlTimeout).Quietly().Run("systemctl", args...)
	if result.Successful() {
		return nil
	}

	output := strings.TrimSpace(strings.Join([]string{result.Output(), result.ErrorOutput()}, "\n"))
	if output == "" {
		return fmt.Errorf("%s失败: %w", operation, result.Error())
	}
	return fmt.Errorf("%s失败: %w, 输出: %s", operation, result.Error(), output)
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

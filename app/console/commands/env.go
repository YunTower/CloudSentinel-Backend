package commands

import (
	"fmt"

	"goravel/app/utils/envfile"
)

// UpdatePortInEnv 更新 .env 文件中的端口配置
func UpdatePortInEnv(port int) error {
	envFile, err := envfile.Path()
	if err != nil {
		return err
	}

	portStr := fmt.Sprintf("%d", port)

	// 更新 APP_PORT
	if _, err := envfile.Update(envFile, "APP_PORT", portStr); err != nil {
		return fmt.Errorf("更新 APP_PORT 失败: %w", err)
	}

	// 更新 APP_URL（需要读取 APP_HOST）
	host, err := envfile.Read(envFile, "APP_HOST")
	if err != nil {
		host = "0.0.0.0"
	}

	appURL := fmt.Sprintf("http://%s:%d", host, port)
	if _, err := envfile.Update(envFile, "APP_URL", appURL); err != nil {
		return fmt.Errorf("更新 APP_URL 失败: %w", err)
	}

	return nil
}

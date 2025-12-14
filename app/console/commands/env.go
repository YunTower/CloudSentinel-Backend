package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GetEnvFilePath 获取 .env 文件路径
func GetEnvFilePath() (string, error) {
	// 首先尝试当前目录
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}

	envFile := filepath.Join(wd, ".env")
	if _, err := os.Stat(envFile); err == nil {
		return envFile, nil
	}

	// 尝试可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	envFile = filepath.Join(exeDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		return envFile, nil
	}

	return "", fmt.Errorf("未找到 .env 文件")
}

// ReadEnvValue 读取 .env 文件中的值
func ReadEnvValue(envFile, key string) (string, error) {
	file, err := os.Open(envFile)
	if err != nil {
		return "", fmt.Errorf("打开 .env 文件失败: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 跳过注释和空行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 匹配 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			value := strings.TrimSpace(parts[1])
			// 移除引号
			value = strings.Trim(value, `"'`)
			return value, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 .env 文件失败: %w", err)
	}

	return "", fmt.Errorf("未找到配置项: %s", key)
}

// UpdateEnvValue 更新 .env 文件中的值
func UpdateEnvValue(envFile, key, value string) error {
	file, err := os.Open(envFile)
	if err != nil {
		return fmt.Errorf("打开 .env 文件失败: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	keyFound := false
	keyPattern := regexp.MustCompile(fmt.Sprintf(`^%s\s*=\s*.*$`, regexp.QuoteMeta(key)))

	for scanner.Scan() {
		line := scanner.Text()
		if keyPattern.MatchString(line) {
			// 更新现有值
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
			keyFound = true
		} else {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取 .env 文件失败: %w", err)
	}

	// 如果未找到，添加新行
	if !keyFound {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	// 写入文件
	if err := os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("写入 .env 文件失败: %w", err)
	}

	return nil
}

// UpdatePortInEnv 更新 .env 文件中的端口配置
func UpdatePortInEnv(port int) error {
	envFile, err := GetEnvFilePath()
	if err != nil {
		return err
	}

	portStr := fmt.Sprintf("%d", port)

	// 更新 APP_PORT
	if err := UpdateEnvValue(envFile, "APP_PORT", portStr); err != nil {
		return fmt.Errorf("更新 APP_PORT 失败: %w", err)
	}

	// 更新 APP_URL（需要读取 APP_HOST）
	host, err := ReadEnvValue(envFile, "APP_HOST")
	if err != nil {
		host = "0.0.0.0"
	}

	appURL := fmt.Sprintf("http://%s:%d", host, port)
	if err := UpdateEnvValue(envFile, "APP_URL", appURL); err != nil {
		return fmt.Errorf("更新 APP_URL 失败: %w", err)
	}

	return nil
}


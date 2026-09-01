// Package envfile 统一 .env 文件的定位、读取与更新。
// 此前 commands、services（TLS 证书）、update_service 各自实现了
// 一套 .env 解析/写入逻辑且行为不一致（路径解析、匹配规则、文件权限），
// 现收敛到本包，全站共用同一套语义。
package envfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Path 返回 .env 文件路径：优先当前工作目录，其次可执行文件所在目录。
func Path() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("获取工作目录失败: %w", err)
	}
	envFile := filepath.Join(wd, ".env")
	if _, err := os.Stat(envFile); err == nil {
		return envFile, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %w", err)
	}
	envFile = filepath.Join(filepath.Dir(exePath), ".env")
	if _, err := os.Stat(envFile); err == nil {
		return envFile, nil
	}

	return "", fmt.Errorf("未找到 .env 文件")
}

// Read 读取 .env 中指定 key 的值（去除两侧引号）；key 不存在或读取失败时返回错误。
func Read(envFile, key string) (string, error) {
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
			return strings.Trim(value, `"'`), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 .env 文件失败: %w", err)
	}

	return "", fmt.Errorf("未找到配置项: %s", key)
}

// Update 更新 .env 中指定 key 的值；key 不存在时追加到文件末尾。
// 返回是否实际写入：已是最新值或文件不可读时返回 false。
// 整行替换并保留其余内容（含注释与空行）。
func Update(envFile, key, value string) (bool, error) {
	file, err := os.Open(envFile)
	if err != nil {
		return false, fmt.Errorf("打开 .env 文件失败: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	keyPattern := regexp.MustCompile(fmt.Sprintf(`^%s\s*=\s*.*$`, regexp.QuoteMeta(key)))
	updated := false
	for scanner.Scan() {
		line := scanner.Text()
		if keyPattern.MatchString(line) {
			// 已是最新值（以首处匹配为准），无需写入
			if !updated && line == fmt.Sprintf("%s=%s", key, value) {
				return false, nil
			}
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
			updated = true
		} else {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("读取 .env 文件失败: %w", err)
	}

	if !updated {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	if err := os.WriteFile(envFile, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return false, fmt.Errorf("写入 .env 文件失败: %w", err)
	}

	return true, nil
}

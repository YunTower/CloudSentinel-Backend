package services

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/facades"
	"github.com/goravel/framework/support/path"
)

// UpdateStatusCallback 更新状态回调接口
type UpdateStatusCallback func(step string, progress int, message string)

// ReleaseInfo 存储从 GitHub API 获取的版本信息
type ReleaseInfo struct {
	TagName           string                 // 原始 tag 名称（如 "v1.0.0"）
	NormalizedTagName string                 // 标准化后的版本号（移除 'v' 前缀）
	VersionType       string                 // 版本类型（如 "release", "beta"）
	Result            map[string]interface{} // 完整的 release 数据
}

// UpdateService 更新服务
type UpdateService struct{}

// NewUpdateService 创建新的更新服务实例
func NewUpdateService() *UpdateService {
	return &UpdateService{}
}

// UpdateOptions 更新选项
type UpdateOptions struct {
	Force          bool                 // 强制更新
	SkipMigration  bool                 // 跳过数据库迁移
	StatusCallback UpdateStatusCallback // 状态更新回调
	ReleaseURL     string               // 发布地址，默认为面板发布地址
}

// ExecuteUpdate 执行更新流程
func (s *UpdateService) ExecuteUpdate(options UpdateOptions) error {
	// 默认发布地址
	releaseURL := options.ReleaseURL
	if releaseURL == "" {
		releaseURL = "https://api.github.com/repos/YunTower/CloudSentinel/releases/latest"
	}

	// 备份文件路径
	var dbBackupPath string
	var binaryBackupPath string
	var needRestore bool

	// 状态更新函数
	setStatus := func(step string, progress int, message string) {
		if options.StatusCallback != nil {
			options.StatusCallback(step, progress, message)
		}
	}

	// 清理函数：失败时恢复备份，成功时删除备份
	defer func() {
		if r := recover(); r != nil {
			facades.Log().Errorf("更新任务发生 panic: %v", r)
			setStatus("error", 0, fmt.Sprintf("更新任务异常: %v", r))
			needRestore = true
		}

		if needRestore {
			facades.Log().Warning("更新失败，正在恢复备份...")
			if dbBackupPath != "" {
				if err := s.RestoreDatabase(dbBackupPath); err != nil {
					facades.Log().Errorf("恢复数据库失败: %v", err)
				} else {
					facades.Log().Info("数据库已恢复")
				}
			}
			if binaryBackupPath != "" {
				if err := s.RestoreBinary(binaryBackupPath); err != nil {
					facades.Log().Errorf("恢复二进制文件失败: %v", err)
				} else {
					facades.Log().Info("二进制文件已恢复")
				}
			}
		} else {
			// 更新成功，删除备份
			if dbBackupPath != "" {
				if err := os.Remove(dbBackupPath); err != nil {
					facades.Log().Warningf("删除数据库备份失败: %v", err)
				}
			}
			if binaryBackupPath != "" {
				if err := os.Remove(binaryBackupPath); err != nil {
					facades.Log().Warningf("删除二进制备份失败: %v", err)
				}
			}
		}
	}()

	// 备份数据库文件
	setStatus("connecting", 5, "正在备份数据库文件...")
	var err error
	dbBackupPath, err = s.BackupDatabase()
	if err != nil {
		setStatus("error", 0, fmt.Sprintf("备份数据库文件失败: %v", err))
		needRestore = true
		return err
	}
	if dbBackupPath != "" {
		facades.Log().Infof("数据库文件已备份到: %s", dbBackupPath)
	}

	// 备份二进制文件
	setStatus("connecting", 8, "正在备份二进制文件...")
	binaryBackupPath, err = s.BackupBinary()
	if err != nil {
		setStatus("error", 0, fmt.Sprintf("备份二进制文件失败: %v", err))
		needRestore = true
		return err
	}
	if binaryBackupPath != "" {
		facades.Log().Infof("二进制文件已备份到: %s", binaryBackupPath)
	}

	// 查询最新发布版本
	setStatus("connecting", 10, "正在连接更新服务器...")
	releaseInfo, err := s.FetchLatestRelease(releaseURL)
	if err != nil {
		setStatus("error", 0, err.Error())
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	// 检查是否需要更新
	if !options.Force {
		currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
		if !s.CompareVersions(currentVersion, releaseInfo.NormalizedTagName) {
			setStatus("error", 0, "当前已是最新版本，无需更新")
			s.CleanupTempFiles()
			needRestore = false // 不需要恢复，因为文件没有被修改
			return nil
		}
	}

	// 获取系统信息
	osType, arch := s.GetSystemInfo()
	setStatus("connecting", 15, fmt.Sprintf("检测到系统: %s-%s", osType, arch))

	// 查找匹配的二进制包
	assets, ok := releaseInfo.Result["assets"].([]interface{})
	if !ok {
		setStatus("error", 0, "未找到发布文件列表")
		s.CleanupTempFiles()
		needRestore = true
		return fmt.Errorf("未找到发布文件列表")
	}

	fileName, downloadUrl := s.FindAssetByArchitecture(assets, osType, arch)
	if fileName == "" {
		setStatus("error", 0, fmt.Sprintf("未找到适用于 %s-%s 的软件包", osType, arch))
		s.CleanupTempFiles()
		needRestore = true
		return fmt.Errorf("未找到适用于 %s-%s 的软件包", osType, arch)
	}

	setStatus("downloading", 0, fmt.Sprintf("找到软件包: %s", fileName))

	// 下载二进制包
	downloadPath := path.Base(fileName)
	if err := s.DownloadFile(downloadUrl, downloadPath, func(progress int) {
		setStatus("downloading", progress, "正在下载软件包...")
	}); err != nil {
		setStatus("error", 0, fmt.Sprintf("下载失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	setStatus("downloading", 100, "软件包下载完成")

	// 查找并下载 SHA256 文件
	sha256FileName, sha256DownloadUrl := s.FindSHA256Asset(assets, osType, arch)
	if sha256FileName == "" {
		setStatus("error", 0, "未找到 SHA256 校验文件")
		s.CleanupTempFiles()
		needRestore = true
		return fmt.Errorf("未找到 SHA256 校验文件")
	}

	sha256Path := path.Base(sha256FileName)
	setStatus("downloading", 65, "正在下载校验文件...")

	if err := s.DownloadFile(sha256DownloadUrl, sha256Path, nil); err != nil {
		setStatus("error", 0, fmt.Sprintf("下载 SHA256 文件失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	setStatus("downloading", 70, "校验文件下载完成")

	// 校验文件完整性
	setStatus("verifying", 85, "正在校验文件完整性...")

	// 读取期望的 SHA256 值
	expectedSHA256, err := s.ReadSHA256File(sha256Path)
	if err != nil {
		setStatus("error", 0, fmt.Sprintf("读取 SHA256 文件失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	// 计算实际 tar.gz 文件的 SHA256 值
	actualSHA256, err := s.CalculateSHA256(downloadPath)
	if err != nil {
		setStatus("error", 0, fmt.Sprintf("计算文件 SHA256 失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	// 比较 SHA256 值
	if !strings.EqualFold(expectedSHA256, actualSHA256) {
		setStatus("error", 0, fmt.Sprintf("文件校验失败: 期望 %s, 实际 %s", expectedSHA256, actualSHA256))
		s.CleanupTempFiles()
		needRestore = true
		return fmt.Errorf("文件校验失败")
	}

	setStatus("verifying", 90, "文件校验通过")

	// 解压 tar.gz 文件
	extractDir := path.Base("update_extract")
	if err := os.RemoveAll(extractDir); err != nil {
		setStatus("error", 0, fmt.Sprintf("清理解压目录失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		setStatus("error", 0, fmt.Sprintf("创建解压目录失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	setStatus("unpacking", 75, "正在解压软件包...")
	if err := s.ExtractTarGz(downloadPath, extractDir); err != nil {
		setStatus("error", 0, fmt.Sprintf("解压失败: %v", err))
		s.CleanupTempFiles()
		needRestore = true
		return err
	}

	setStatus("unpacking", 80, "解压完成")

	// 查找解压后的二进制文件
	binaryName := fmt.Sprintf("dashboard-%s-%s", osType, arch)
	if osType == "windows" {
		binaryName = fmt.Sprintf("dashboard-%s-%s.exe", osType, arch)
	}
	extractedBinaryPath := filepath.Join(extractDir, binaryName)

	// 检查文件是否存在
	if _, err := os.Stat(extractedBinaryPath); os.IsNotExist(err) {
		// 尝试在子目录中查找
		files, err := os.ReadDir(extractDir)
		if err != nil {
			setStatus("error", 0, fmt.Sprintf("读取解压目录失败: %v", err))
			s.CleanupTempFiles()
			needRestore = true
			return err
		}

		found := false
		for _, file := range files {
			if file.IsDir() {
				subPath := filepath.Join(extractDir, file.Name(), binaryName)
				if _, err := os.Stat(subPath); err == nil {
					extractedBinaryPath = subPath
					found = true
					break
				}
			}
		}

		if !found {
			setStatus("error", 0, "解压后未找到二进制文件")
			s.CleanupTempFiles()
			needRestore = true
			return fmt.Errorf("解压后未找到二进制文件")
		}
	}

	setStatus("verifying", 90, "文件校验通过")

	// 删除 SHA256 文件
	if err := os.Remove(sha256Path); err != nil {
		facades.Log().Warningf("删除 SHA256 文件失败: %v", err)
	}

	// 替换文件
	setStatus("unpacking", 95, "正在替换文件...")

	// 获取当前可执行文件路径
	currentExecPath, err := os.Executable()
	if err != nil {
		setStatus("error", 0, fmt.Sprintf("获取当前可执行文件路径失败: %v", err))
		needRestore = true
		return err
	}

	// 替换文件
	if err := s.CopyFile(extractedBinaryPath, currentExecPath); err != nil {
		setStatus("error", 0, fmt.Sprintf("替换文件失败: %v", err))
		needRestore = true
		return err
	}

	// 设置可执行权限
	if err := os.Chmod(currentExecPath, 0755); err != nil {
		facades.Log().Warningf("设置可执行权限失败: %v", err)
	}

	// 验证文件替换是否成功
	if _, err := os.Stat(currentExecPath); err != nil {
		setStatus("error", 0, fmt.Sprintf("验证替换文件失败: %v", err))
		needRestore = true
		return fmt.Errorf("验证替换文件失败: %v", err)
	}

	// 验证文件大小是否匹配
	extractedInfo, err := os.Stat(extractedBinaryPath)
	if err == nil {
		replacedInfo, err := os.Stat(currentExecPath)
		if err == nil {
			if extractedInfo.Size() != replacedInfo.Size() {
				setStatus("error", 0, "文件替换后大小不匹配")
				needRestore = true
				return fmt.Errorf("文件替换后大小不匹配")
			}
		}
	}

	setStatus("unpacking", 98, "文件替换完成并已验证")

	// 清理解压目录和下载的压缩文件
	facades.Log().Infof("开始清理临时文件: 解压目录=%s, 压缩文件=%s", extractDir, downloadPath)
	s.CleanupTempFiles(extractDir, downloadPath)

	// 执行数据库迁移
	if !options.SkipMigration {
		setStatus("migrating", 99, "正在执行数据库迁移...")
		if err := s.RunMigrations(); err != nil {
			facades.Log().Errorf("执行数据库迁移失败: %v", err)
			setStatus("migrating", 99, fmt.Sprintf("数据库迁移警告: %v（更新将继续）", err))
		} else {
			setStatus("migrating", 99, "数据库迁移完成")
		}
	}

	// 标记更新成功，defer 会清理备份文件
	needRestore = false

	setStatus("completed", 100, "更新完成！")

	// 延迟一下，确保状态已保存到缓存
	time.Sleep(2 * time.Second)

	// 重启程序
	setStatus("restarting", 99, "正在重启服务...")

	time.Sleep(1 * time.Second)

	// 执行重启
	if err := s.RestartApplication(); err != nil {
		facades.Log().Errorf("重启应用失败: %v", err)
		setStatus("error", 0, fmt.Sprintf("重启失败: %v", err))
		needRestore = true
		return err
	}

	return nil
}

// GetDatabasePath 获取数据库文件路径
func (s *UpdateService) GetDatabasePath() (string, error) {
	// 从配置获取数据库路径
	dbName := facades.Config().GetString("database.connections.sqlite.database", "forge")

	// 如果是相对路径，尝试从 .env 文件读取
	if !filepath.IsAbs(dbName) {
		// 尝试当前工作目录
		wd, err := os.Getwd()
		if err == nil {
			envFile := filepath.Join(wd, ".env")
			if _, err := os.Stat(envFile); err == nil {
				// 读取 .env 文件
				data, err := os.ReadFile(envFile)
				if err == nil {
					lines := strings.Split(string(data), "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "DB_DATABASE=") {
							value := strings.TrimPrefix(line, "DB_DATABASE=")
							value = strings.Trim(value, `"'`)
							if value != "" {
								dbName = value
								break
							}
						}
					}
				}
			}
		}

		// 如果仍然是相对路径，尝试在当前目录或可执行文件目录查找
		if !filepath.IsAbs(dbName) {
			// 尝试当前工作目录
			wd, err := os.Getwd()
			if err == nil {
				dbPath := filepath.Join(wd, dbName)
				if _, err := os.Stat(dbPath); err == nil {
					return dbPath, nil
				}
			}

			// 尝试可执行文件目录
			exePath, err := os.Executable()
			if err == nil {
				exeDir := filepath.Dir(exePath)
				dbPath := filepath.Join(exeDir, dbName)
				if _, err := os.Stat(dbPath); err == nil {
					return dbPath, nil
				}
			}
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(dbName); os.IsNotExist(err) {
		// 数据库文件不存在，返回空字符串但不报错
		return "", nil
	}

	return dbName, nil
}

// BackupDatabase 备份数据库文件
func (s *UpdateService) BackupDatabase() (string, error) {
	dbPath, err := s.GetDatabasePath()
	if err != nil {
		return "", fmt.Errorf("获取数据库路径失败: %w", err)
	}

	if dbPath == "" {
		// 数据库文件不存在，返回空字符串但不报错
		return "", nil
	}

	backupPath := fmt.Sprintf("%s.backup.%d", dbPath, time.Now().Unix())
	if err := s.CopyFile(dbPath, backupPath); err != nil {
		return "", fmt.Errorf("备份数据库文件失败: %w", err)
	}

	return backupPath, nil
}

// BackupBinary 备份二进制文件
func (s *UpdateService) BackupBinary() (string, error) {
	currentExecPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取当前可执行文件路径失败: %w", err)
	}

	backupPath := fmt.Sprintf("%s.backup.%d", currentExecPath, time.Now().Unix())
	if err := s.CopyFile(currentExecPath, backupPath); err != nil {
		return "", fmt.Errorf("备份二进制文件失败: %w", err)
	}

	return backupPath, nil
}

// RestoreDatabase 恢复数据库文件
func (s *UpdateService) RestoreDatabase(backupPath string) error {
	if backupPath == "" {
		return nil // 没有备份，无需恢复
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	dbPath, err := s.GetDatabasePath()
	if err != nil {
		return fmt.Errorf("获取数据库路径失败: %w", err)
	}

	if dbPath == "" {
		return fmt.Errorf("数据库路径为空，无法恢复")
	}

	if err := s.CopyFile(backupPath, dbPath); err != nil {
		return fmt.Errorf("恢复数据库文件失败: %w", err)
	}

	return nil
}

// RestoreBinary 恢复二进制文件
func (s *UpdateService) RestoreBinary(backupPath string) error {
	if backupPath == "" {
		return nil // 没有备份，无需恢复
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在: %s", backupPath)
	}

	currentExecPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %w", err)
	}

	if err := s.CopyFile(backupPath, currentExecPath); err != nil {
		return fmt.Errorf("恢复二进制文件失败: %w", err)
	}

	// 设置可执行权限
	if err := os.Chmod(currentExecPath, 0755); err != nil {
		facades.Log().Warningf("设置可执行权限失败: %v", err)
	}

	return nil
}

// CompareVersions 比较版本号，返回 true 表示需要更新
func (s *UpdateService) CompareVersions(currentVersion, latestVersion string) bool {
	// 解析版本号
	currentVer, currentType, currentPreReleaseNum := s.ParseVersion(currentVersion)
	latestVer, latestType, latestPreReleaseNum := s.ParseVersion(latestVersion)

	// 比较主版本号
	currentNums := strings.Split(currentVer, ".")
	latestNums := strings.Split(latestVer, ".")

	maxLen := len(currentNums)
	if len(latestNums) > maxLen {
		maxLen = len(latestNums)
	}

	for i := 0; i < maxLen; i++ {
		currentNum := 0
		latestNum := 0

		if i < len(currentNums) {
			currentNum, _ = strconv.Atoi(currentNums[i])
		}
		if i < len(latestNums) {
			latestNum, _ = strconv.Atoi(latestNums[i])
		}

		if latestNum > currentNum {
			return true
		}
		if latestNum < currentNum {
			return false
		}
	}

	// 版本号相同，比较预发布类型优先级
	versionTypePriority := map[string]int{
		"dev":     0,
		"alpha":   1,
		"beta":    2,
		"rc":      3,
		"release": 4,
	}

	currentPriority := versionTypePriority[currentType]
	latestPriority := versionTypePriority[latestType]

	// 如果类型优先级不同，直接比较优先级
	if latestPriority != currentPriority {
		return latestPriority > currentPriority
	}

	// 类型优先级相同，比较预发布版本序号（仅对非 release 版本）
	if currentType != "release" && latestType != "release" {
		return latestPreReleaseNum > currentPreReleaseNum
	}

	// 如果一个是 release，另一个不是，release 优先级更高
	if currentType == "release" && latestType != "release" {
		return false
	}
	if currentType != "release" && latestType == "release" {
		return true
	}

	// 都是 release 或类型相同且序号相同，不需要更新
	return false
}

// ParseVersion 解析版本号字符串
func (s *UpdateService) ParseVersion(version string) (string, string, int) {
	// 移除开头的 'v' 前缀
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	// 分割版本号和预发布标识
	parts := strings.SplitN(version, "-", 2)
	mainVersion := parts[0]
	preRelease := "release"
	preReleaseNum := 0

	if len(parts) > 1 {
		preRelease = parts[1]
		// 尝试提取预发布版本序号（如 beta.1 中的 1）
		preReleaseParts := strings.Split(preRelease, ".")
		if len(preReleaseParts) > 1 {
			// 提取类型（如 "beta"）
			preRelease = preReleaseParts[0]
			// 提取序号（如 "1"）
			if num, err := strconv.Atoi(preReleaseParts[1]); err == nil {
				preReleaseNum = num
			}
		}
	}

	return mainVersion, preRelease, preReleaseNum
}

// FetchLatestRelease 从 GitHub API 获取最新版本信息
func (s *UpdateService) FetchLatestRelease(releaseUrl string) (*ReleaseInfo, error) {
	response, requestErr := facades.Http().Get(releaseUrl)
	if requestErr != nil {
		return nil, fmt.Errorf("请求最新版本信息失败: %v", requestErr)
	}

	responseBody, responseErr := response.Body()
	if responseErr != nil {
		return nil, fmt.Errorf("读取最新版本信息失败: %v", responseErr)
	}

	if response.Status() == 404 {
		return nil, fmt.Errorf("未找到最新的版本信息")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(responseBody), &result); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %v", err)
	}

	tagName, ok := result["tag_name"].(string)
	if !ok {
		return nil, fmt.Errorf("版本信息格式错误: tag_name 字段缺失或类型不正确")
	}

	// 格式化版本号（移除 'v' 前缀）
	normalizedTagName := tagName
	if len(tagName) > 0 && tagName[0] == 'v' {
		normalizedTagName = tagName[1:]
	}

	// 使用 ParseVersion 提取版本类型
	_, versionType, _ := s.ParseVersion(normalizedTagName)

	return &ReleaseInfo{
		TagName:           tagName,
		NormalizedTagName: normalizedTagName,
		VersionType:       versionType,
		Result:            result,
	}, nil
}

// GetSystemInfo 获取系统信息
func (s *UpdateService) GetSystemInfo() (osType, arch string) {
	osType = runtime.GOOS
	arch = runtime.GOARCH

	// 标准化架构名称
	if arch == "amd64" {
		arch = "amd64"
	} else if arch == "386" {
		arch = "386"
	} else if arch == "arm64" {
		arch = "arm64"
	} else if arch == "arm" {
		arch = "arm"
	}

	return osType, arch
}

// FindAssetByArchitecture 在 assets 中查找匹配的二进制包
func (s *UpdateService) FindAssetByArchitecture(assets []interface{}, osType, arch string) (string, string) {
	expectedName := fmt.Sprintf("dashboard-%s-%s.tar.gz", osType, arch)

	for _, asset := range assets {
		assetMap, ok := asset.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := assetMap["name"].(string)
		if !ok {
			continue
		}

		if name == expectedName {
			var downloadUrl string

			if url, ok := assetMap["browser_download_url"].(string); ok {
				downloadUrl = url
			}

			if downloadUrl != "" {
				return name, downloadUrl
			}
		}
	}

	return "", ""
}

// FindSHA256Asset 在 assets 中查找匹配的 SHA256 文件
func (s *UpdateService) FindSHA256Asset(assets []interface{}, osType, arch string) (string, string) {
	expectedName := fmt.Sprintf("dashboard-%s-%s.sha256", osType, arch)

	for _, asset := range assets {
		assetMap, ok := asset.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := assetMap["name"].(string)
		if !ok {
			continue
		}

		if name == expectedName {
			var downloadUrl string
			if url, ok := assetMap["browser_download_url"].(string); ok {
				downloadUrl = url
			}

			if downloadUrl != "" {
				return name, downloadUrl
			}
		}
	}

	return "", ""
}

// DownloadFile 下载文件
func (s *UpdateService) DownloadFile(url, filePath string, progressCallback func(int)) error {
	response, err := facades.Http().Get(url)
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}

	body, err := response.Body()
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	// 创建目录
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 创建文件
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			facades.Log().Warningf("关闭文件失败: %v", closeErr)
		}
	}()

	// 流式写入并计算进度
	bodyBytes := []byte(body)
	totalSize := len(bodyBytes)
	chunkSize := 8192 // 8KB chunks
	written := 0

	for written < totalSize {
		end := written + chunkSize
		if end > totalSize {
			end = totalSize
		}

		n, err := out.Write(bodyBytes[written:end])
		if err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}

		written += n

		// 计算进度并回调
		if progressCallback != nil {
			progress := int(float64(written) / float64(totalSize) * 100)
			progressCallback(progress)
		}
	}

	return nil
}

// ExtractTarGz 解压 tar.gz 文件
func (s *UpdateService) ExtractTarGz(tarGzPath, destDir string) error {
	// 打开 tar.gz 文件
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("打开压缩文件失败: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			facades.Log().Warningf("关闭压缩文件失败: %v", closeErr)
		}
	}()

	// 创建 gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("创建 gzip reader 失败: %v", err)
	}
	defer func() {
		if closeErr := gzReader.Close(); closeErr != nil {
			facades.Log().Warningf("关闭 gzip reader 失败: %v", closeErr)
		}
	}()

	// 创建 tar reader
	tarReader := tar.NewReader(gzReader)

	// 解压所有文件
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 文件失败: %v", err)
		}

		// 构建目标文件路径
		targetPath := filepath.Join(destDir, header.Name)

		// 处理目录
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		// 处理文件
		if header.Typeflag == tar.TypeReg {
			// 确保目录存在
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}

			// 创建文件
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("创建文件失败: %v", err)
			}

			// 复制文件内容
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("复制文件内容失败: %v", err)
			}

			// 立即关闭文件
			if err := outFile.Close(); err != nil {
				return fmt.Errorf("关闭解压文件失败: %v", err)
			}

			// 设置文件权限
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("设置文件权限失败: %v", err)
			}
		}
	}

	return nil
}

// CalculateSHA256 计算文件的 SHA256 值
func (s *UpdateService) CalculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			facades.Log().Warningf("关闭文件失败: %v", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算哈希失败: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CopyFile 复制文件
func (s *UpdateService) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭源文件失败: %v", closeErr)
		}
	}()

	// 在 Linux 系统上，如果目标文件存在且可能正在运行，使用原子替换
	if runtime.GOOS != "windows" {
		// 检查目标文件是否存在
		if _, err := os.Stat(dst); err == nil {
			// 目标文件存在，使用原子替换方式
			tempDst := dst + ".new"
			destFile, err := os.Create(tempDst)
			if err != nil {
				return fmt.Errorf("创建临时文件失败: %v", err)
			}

			// 复制文件内容
			if _, err := io.Copy(destFile, sourceFile); err != nil {
				destFile.Close()
				os.Remove(tempDst)
				return fmt.Errorf("复制文件内容失败: %v", err)
			}

			// 确保数据写入磁盘
			if err := destFile.Sync(); err != nil {
				destFile.Close()
				os.Remove(tempDst)
				return fmt.Errorf("同步文件失败: %v", err)
			}

			// 关闭文件
			if err := destFile.Close(); err != nil {
				os.Remove(tempDst)
				return fmt.Errorf("关闭临时文件失败: %v", err)
			}

			// 设置可执行权限（如果需要）
			if err := os.Chmod(tempDst, 0755); err != nil {
				facades.Log().Warningf("设置临时文件权限失败: %v", err)
			}

			// 使用原子替换
			if err := os.Rename(tempDst, dst); err != nil {
				os.Remove(tempDst)
				return fmt.Errorf("原子替换文件失败: %v", err)
			}

			return nil
		}
	}

	// Windows 系统或目标文件不存在，使用常规方式
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭目标文件失败: %v", closeErr)
		}
	}()

	// 复制文件内容
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	// 确保数据写入磁盘
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("同步文件失败: %v", err)
	}

	return nil
}

// ReadSHA256File 读取 SHA256 文件内容
func (s *UpdateService) ReadSHA256File(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取 SHA256 文件失败: %v", err)
	}

	// SHA256 文件格式通常是 "hash  filename" 或只有 "hash"
	content := strings.TrimSpace(string(data))
	parts := strings.Fields(content)
	if len(parts) > 0 {
		return parts[0], nil
	}

	return "", fmt.Errorf("SHA256 文件格式错误")
}

// CleanupTempFiles 清理更新过程中的临时文件
func (s *UpdateService) CleanupTempFiles(files ...string) {
	// 如果提供了具体的文件路径，优先清理这些文件
	if len(files) > 0 {
		for _, file := range files {
			if file == "" {
				continue
			}
			if err := os.RemoveAll(file); err != nil {
				facades.Log().Warningf("删除临时文件失败 (%s): %v", file, err)
			} else {
				facades.Log().Infof("已清理临时文件: %s", file)
			}
		}
	}

	// 保留原有的模式匹配清理逻辑作为后备
	tempDir := os.TempDir()
	patterns := []string{
		"dashboard-*.tar.gz",
		"dashboard-*.sha256",
		"update_extract",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
		if err != nil {
			facades.Log().Warningf("清理临时文件失败 (pattern: %s): %v", pattern, err)
			continue
		}

		for _, match := range matches {
			// 跳过已经在上面清理过的文件
			skip := false
			for _, file := range files {
				if match == file {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			if err := os.RemoveAll(match); err != nil {
				facades.Log().Warningf("删除临时文件失败 (%s): %v", match, err)
			} else {
				facades.Log().Infof("已清理临时文件: %s", match)
			}
		}
	}
}

// RunMigrations 执行数据库迁移
func (s *UpdateService) RunMigrations() error {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 获取可执行文件所在目录
	execDir := filepath.Dir(execPath)

	cmd := exec.Command(execPath, "artisan", "migrate")

	// 设置工作目录
	cmd.Dir = execDir

	// 设置环境变量
	cmd.Env = os.Environ()

	// 捕获输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	facades.Log().Info("开始执行数据库迁移...")
	if err := cmd.Run(); err != nil {
		output := stdout.String()
		errOutput := stderr.String()
		facades.Log().Errorf("执行迁移命令失败: %v\n标准输出: %s\n错误输出: %s", err, output, errOutput)
		return fmt.Errorf("执行迁移命令失败: %v\n输出: %s\n错误: %s", err, output, errOutput)
	}

	output := stdout.String()
	if output != "" {
		facades.Log().Infof("迁移执行输出: %s", output)
	}

	facades.Log().Info("数据库迁移执行完成")
	return nil
}

// RestartApplication 重启应用程序
func (s *UpdateService) RestartApplication() error {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	// 获取当前进程的 PID
	pid := os.Getpid()

	// 构建重启命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows
		cmd = exec.Command("cmd", "/C", "timeout", "/t", "2", "/nobreak", ">nul", "&", execPath)
	} else {
		// Linux/Unix
		cmd = exec.Command("sh", "-c", fmt.Sprintf("sleep 2 && %s &", execPath))
	}

	// 设置工作目录
	cmd.Dir = filepath.Dir(execPath)

	// 设置环境变量
	cmd.Env = os.Environ()

	// 启动新进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动新进程失败: %v", err)
	}

	facades.Log().Infof("新进程已启动，PID: %d，正在终止当前进程 PID: %d", cmd.Process.Pid, pid)

	if runtime.GOOS == "windows" {
		time.Sleep(3 * time.Second)
	} else {
		time.Sleep(2 * time.Second)
	}

	// 终止当前进程
	if runtime.GOOS == "windows" {
		// Windows
		killCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		if err := killCmd.Run(); err != nil {
			facades.Log().Warningf("终止当前进程失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}
	} else {
		// Linux/Unix
		process, err := os.FindProcess(pid)
		if err != nil {
			facades.Log().Warningf("查找当前进程失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}

		if err := process.Signal(os.Interrupt); err != nil {
			facades.Log().Warningf("发送终止信号失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}

		// 等待进程退出，如果 5 秒后还没退出，强制退出
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}

	return nil
}

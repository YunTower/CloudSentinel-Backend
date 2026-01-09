package controllers

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

	"goravel/app/services"
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
	"github.com/goravel/framework/support/path"
)

type UpdateController struct{}

type UpdateStatus struct {
	Step     string `json:"step"`     // connecting, downloading, verifying, unpacking, restarting, completed, error
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

// ReleaseInfo 存储从 GitHub API 获取的版本信息
type ReleaseInfo struct {
	TagName           string                 // 原始 tag 名称（如 "v1.0.0"）
	NormalizedTagName string                 // 标准化后的版本号（移除 'v' 前缀）
	VersionType       string                 // 版本类型（如 "release", "beta"）
	Result            map[string]interface{} // 完整的 release 数据
}

// releaseUrls 版本发布地址
var releaseUrls = "https://api.github.com/repos/YunTower/CloudSentinel/releases/latest"

// agentReleaseUrls Agent 版本发布地址
var agentReleaseUrls = "https://api.github.com/repos/YunTower/CloudSentinel-Agent/releases/latest"

func NewUpdateController() *UpdateController {
	return &UpdateController{}
}

// Status 获取更新状态
func (r *UpdateController) Status(ctx http.Context) http.Response {
	status := UpdateStatus{
		Step:     "pending",
		Progress: 0,
		Message:  "",
	}

	// 从缓存获取状态
	if facades.Cache().Has("update_status") {
		cachedValue := facades.Cache().Get("update_status", status)
		if cachedValue != nil {
			// 尝试类型断言
			if cachedStatus, ok := cachedValue.(UpdateStatus); ok {
				status = cachedStatus
			} else {
				// 如果类型断言失败，尝试通过 JSON 反序列化
				facades.Log().Warningf("缓存状态类型不匹配，尝试其他方式读取")
				var cachedStatus UpdateStatus
				if err := facades.Cache().Get("update_status", &cachedStatus); err == nil {
					status = cachedStatus
				} else {
					facades.Log().Errorf("读取缓存状态失败: %v", err)
				}
			}
		}
	}

	return utils.SuccessResponse(ctx, "success", status)
}

// compareVersions 比较版本号，返回 true 表示需要更新
// 支持格式：v0.0.1, v0.0.1-release, v0.0.1-beta.1, v0.0.1-rc.2 等
func (r *UpdateController) compareVersions(currentVersion, latestVersion string) bool {
	// 解析版本号
	currentVer, currentType, currentPreReleaseNum := r.parseVersion(currentVersion)
	latestVer, latestType, latestPreReleaseNum := r.parseVersion(latestVersion)

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

// parseVersion 解析版本号字符串
// 返回：主版本号、版本类型、预发布版本序号
// 示例：parseVersion("0.0.1-beta.1") -> ("0.0.1", "beta", 1)
//
//	parseVersion("0.0.1-release") -> ("0.0.1", "release", 0)
func (r *UpdateController) parseVersion(version string) (string, string, int) {
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

// fetchLatestRelease 从 GitHub API 获取最新版本信息
func (r *UpdateController) fetchLatestRelease(releaseUrl string) (*ReleaseInfo, error) {
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

	// 使用 parseVersion 提取版本类型
	_, versionType, _ := r.parseVersion(normalizedTagName)

	return &ReleaseInfo{
		TagName:           tagName,
		NormalizedTagName: normalizedTagName,
		VersionType:       versionType,
		Result:            result,
	}, nil
}

// getSystemInfo 获取系统信息
func (r *UpdateController) getSystemInfo() (osType, arch string) {
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

// findAssetByArchitecture 在 assets 中查找匹配的二进制包
func (r *UpdateController) findAssetByArchitecture(assets []interface{}, osType, arch string) (string, string) {
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

// findSHA256Asset 在 assets 中查找匹配的 SHA256 文件
func (r *UpdateController) findSHA256Asset(assets []interface{}, osType, arch string) (string, string) {
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

// downloadFile 下载文件
func (r *UpdateController) downloadFile(url, filePath string, progressCallback func(int)) error {
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

// extractTarGz 解压 tar.gz 文件
func (r *UpdateController) extractTarGz(tarGzPath, destDir string) error {
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

// calculateSHA256 计算文件的 SHA256 值
func (r *UpdateController) calculateSHA256(filePath string) (string, error) {
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

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭源文件失败: %v", closeErr)
		}
	}()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			facades.Log().Warningf("关闭目标文件失败: %v", closeErr)
		}
	}()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// readSHA256File 读取 SHA256 文件内容
func (r *UpdateController) readSHA256File(filePath string) (string, error) {
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

// cleanupTempFiles 清理更新过程中的临时文件
func (r *UpdateController) cleanupTempFiles() {
	tempDir := os.TempDir()

	// 清理可能的临时文件
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
			if err := os.RemoveAll(match); err != nil {
				facades.Log().Warningf("删除临时文件失败 (%s): %v", match, err)
			} else {
				facades.Log().Infof("已清理临时文件: %s", match)
			}
		}
	}
}

// UpdatePanel 执行面板更新
func (r *UpdateController) UpdatePanel(ctx http.Context) http.Response {
	// 检查是否已经在更新中
	// 只有在进行中的状态（非 completed、error、pending）才阻止新的更新
	if facades.Cache().Has("update_status") {
		cachedValue := facades.Cache().Get("update_status", nil)
		if cachedValue != nil {
			var status UpdateStatus
			if cachedStatus, ok := cachedValue.(UpdateStatus); ok {
				status = cachedStatus
			} else {
				// 尝试通过指针方式获取
				if err := facades.Cache().Get("update_status", &status); err != nil {
					// 如果读取失败，清除缓存，允许重新开始
					facades.Cache().Forget("update_status")
				}
			}

			// 只有在进行中的状态才阻止新的更新
			activeSteps := map[string]bool{
				"connecting":  true,
				"downloading": true,
				"verifying":   true,
				"unpacking":   true,
				"restarting":  true,
			}

			if activeSteps[status.Step] {
				return utils.ErrorResponse(ctx, 400, "更新已在进行中", "UPDATE_IN_PROGRESS")
			}

			// 如果是 error 或 completed 状态，清除旧状态，允许重新开始
			if status.Step == "error" || status.Step == "completed" {
				facades.Cache().Forget("update_status")
				facades.Log().Infof("清除旧的更新状态 (%s)，允许重新开始更新", status.Step)
			}
		}
	}

	// 设置初始状态
	initialStatus := UpdateStatus{
		Step:     "connecting",
		Progress: 0,
		Message:  "正在初始化更新任务...",
	}
	if err := facades.Cache().Put("update_status", initialStatus, 10*time.Minute); err != nil {
		facades.Log().Errorf("设置更新状态失败: %v", err)
		return utils.ErrorResponseWithError(ctx, 500, "设置更新状态失败", err, "CACHE_ERROR")
	}

	// 启动异步更新任务
	go func() {
		setStatus := func(step string, progress int, message string) {
			status := UpdateStatus{
				Step:     step,
				Progress: progress,
				Message:  message,
			}
			if err := facades.Cache().Put("update_status", status, 10*time.Minute); err != nil {
				facades.Log().Errorf("更新状态失败: %v", err)
			}
		}

		setStatus("connecting", 0, "正在连接更新服务器...")

		// 清理可能存在的临时文件
		defer func() {
			if r := recover(); r != nil {
				facades.Log().Errorf("更新任务发生 panic: %v", r)
				setStatus("error", 0, fmt.Sprintf("更新任务异常: %v", r))
			}
		}()

		// 查询最新发布版本
		releaseInfo, err := r.fetchLatestRelease(releaseUrls)
		if err != nil {
			setStatus("error", 0, err.Error())
			r.cleanupTempFiles()
			return
		}

		// 检查是否需要更新
		currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
		if !r.compareVersions(currentVersion, releaseInfo.NormalizedTagName) {
			setStatus("error", 0, "当前已是最新版本，无需更新")
			r.cleanupTempFiles()
			return
		}

		// 获取系统信息
		osType, arch := r.getSystemInfo()
		setStatus("connecting", 10, fmt.Sprintf("检测到系统: %s-%s", osType, arch))

		// 查找匹配的二进制包
		assets, ok := releaseInfo.Result["assets"].([]interface{})
		if !ok {
			setStatus("error", 0, "未找到发布文件列表")
			r.cleanupTempFiles()
			return
		}

		fileName, downloadUrl := r.findAssetByArchitecture(assets, osType, arch)
		if fileName == "" {
			setStatus("error", 0, fmt.Sprintf("未找到适用于 %s-%s 的软件包", osType, arch))
			r.cleanupTempFiles()
			return
		}

		setStatus("downloading", 0, fmt.Sprintf("找到软件包: %s", fileName))

		// 下载二进制包
		downloadPath := path.Base(fileName)
		if err := r.downloadFile(downloadUrl, downloadPath, func(progress int) {
			setStatus("downloading", progress, "正在下载软件包...")
		}); err != nil {
			setStatus("error", 0, fmt.Sprintf("下载失败: %v", err))
			r.cleanupTempFiles()
			return
		}

		setStatus("downloading", 100, "软件包下载完成")

		// 查找并下载 SHA256 文件
		sha256FileName, sha256DownloadUrl := r.findSHA256Asset(assets, osType, arch)
		if sha256FileName == "" {
			setStatus("error", 0, "未找到 SHA256 校验文件")
			r.cleanupTempFiles()
			return
		}

		sha256Path := path.Base(sha256FileName)
		setStatus("downloading", 65, "正在下载校验文件...")

		if err := r.downloadFile(sha256DownloadUrl, sha256Path, nil); err != nil {
			setStatus("error", 0, fmt.Sprintf("下载 SHA256 文件失败: %v", err))
			r.cleanupTempFiles()
			return
		}

		setStatus("downloading", 70, "校验文件下载完成")

		// 校验文件完整性
		setStatus("verifying", 85, "正在校验文件完整性...")

		// 读取期望的 SHA256 值
		expectedSHA256, err := r.readSHA256File(sha256Path)
		if err != nil {
			setStatus("error", 0, fmt.Sprintf("读取 SHA256 文件失败: %v", err))
			r.cleanupTempFiles()
			return
		}

		// 计算实际 tar.gz 文件的 SHA256 值
		actualSHA256, err := r.calculateSHA256(downloadPath)
		if err != nil {
			setStatus("error", 0, fmt.Sprintf("计算文件 SHA256 失败: %v", err))
			r.cleanupTempFiles()
			return
		}

		// 比较 SHA256 值
		if !strings.EqualFold(expectedSHA256, actualSHA256) {
			setStatus("error", 0, fmt.Sprintf("文件校验失败: 期望 %s, 实际 %s", expectedSHA256, actualSHA256))
			r.cleanupTempFiles()
			return
		}

		setStatus("verifying", 90, "文件校验通过")

		// 解压 tar.gz 文件
		extractDir := path.Base("update_extract")
		if err := os.RemoveAll(extractDir); err != nil {
			setStatus("error", 0, fmt.Sprintf("清理解压目录失败: %v", err))
			r.cleanupTempFiles()
			return
		}
		if err := os.MkdirAll(extractDir, 0755); err != nil {
			setStatus("error", 0, fmt.Sprintf("创建解压目录失败: %v", err))
			r.cleanupTempFiles()
			return
		}

		setStatus("unpacking", 75, "正在解压软件包...")
		if err := r.extractTarGz(downloadPath, extractDir); err != nil {
			setStatus("error", 0, fmt.Sprintf("解压失败: %v", err))
			r.cleanupTempFiles()
			return
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
				r.cleanupTempFiles()
				return
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
				r.cleanupTempFiles()
				return
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
			return
		}

		// 备份当前文件
		backupPath := currentExecPath + ".backup"
		if err := copyFile(currentExecPath, backupPath); err != nil {
			setStatus("error", 0, fmt.Sprintf("备份当前文件失败: %v", err))
			return
		}

		// 替换文件
		if err := copyFile(extractedBinaryPath, currentExecPath); err != nil {
			// 恢复备份
			err := copyFile(backupPath, currentExecPath)
			if err != nil {
				return
			}
			setStatus("error", 0, fmt.Sprintf("替换文件失败: %v", err))
			return
		}

		// 设置可执行权限
		if err := os.Chmod(currentExecPath, 0755); err != nil {
			facades.Log().Warningf("设置可执行权限失败: %v", err)
		}

		setStatus("unpacking", 98, "文件替换完成")

		// 执行数据库迁移
		setStatus("migrating", 99, "正在执行数据库迁移...")
		if err := r.runMigrations(); err != nil {
			facades.Log().Errorf("执行数据库迁移失败: %v", err)
			setStatus("migrating", 99, fmt.Sprintf("数据库迁移警告: %v（更新将继续）", err))
		} else {
			setStatus("migrating", 99, "数据库迁移完成")
		}

		// 重启程序
		setStatus("restarting", 99, "正在重启服务...")

		// 延迟一下，确保状态已保存
		time.Sleep(1 * time.Second)

		// 执行重启
		if err := r.restartApplication(); err != nil {
			facades.Log().Errorf("重启应用失败: %v", err)
			setStatus("error", 0, fmt.Sprintf("重启失败: %v", err))
			return
		}

		setStatus("completed", 100, "更新完成！")
		// 保持完成状态一段时间，以便前端读取
		time.Sleep(1 * time.Minute)
		facades.Cache().Forget("update_status")
	}()

	return utils.SuccessResponse(ctx, "更新任务已启动")
}

// checkVersion 检查版本信息的公共方法
func (r *UpdateController) checkVersion(ctx http.Context, releaseUrl string, includeCurrentVersion bool) http.Response {
	releaseInfo, err := r.fetchLatestRelease(releaseUrl)
	if err != nil {
		// 根据错误类型返回相应的 HTTP 状态码
		if strings.Contains(err.Error(), "未找到") {
			return utils.ErrorResponse(ctx, 404, "未找到最新的版本信息，改天再试试吧", "LATEST_VERSION_NOT_FOUND")
		}
		return utils.ErrorResponseWithError(ctx, 500, err.Error(), err, "FETCH_RELEASE_FAILED")
	}

	// 格式化发布时间
	var publishTime string
	if createdAt, ok := releaseInfo.Result["created_at"].(string); ok && createdAt != "" {
		parsedTime, parseErr := time.Parse(time.RFC3339, createdAt)
		if parseErr == nil {
			publishTime = parsedTime.Format("2006-01-02 15:04:05")
		} else {
			publishTime = createdAt
		}
	}

	data := map[string]any{
		"latest_version":      releaseInfo.NormalizedTagName,
		"latest_version_type": releaseInfo.VersionType,
		"publish_time":        publishTime,
		"change_log":          releaseInfo.Result["body"],
	}

	// 如果需要包含当前版本信息
	if includeCurrentVersion {
		currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
		_, currentVersionType, _ := r.parseVersion(currentVersion)
		data["current_version"] = currentVersion
		data["current_version_type"] = currentVersionType
	}

	return utils.SuccessResponse(ctx, "success", data)
}

func (r *UpdateController) Check(ctx http.Context) http.Response {
	return r.checkVersion(ctx, releaseUrls, true)
}

// CheckAgent 检查 Agent 最新版本
func (r *UpdateController) CheckAgent(ctx http.Context) http.Response {
	return r.checkVersion(ctx, agentReleaseUrls, false)
}

// UpdateAgent 更新服务器 Agent
func (r *UpdateController) UpdateAgent(ctx http.Context) http.Response {
	serverID := ctx.Request().Route("id")
	if serverID == "" {
		return utils.ErrorResponse(ctx, http.StatusBadRequest, "缺少服务器ID", "MISSING_SERVER_ID")
	}

	// 获取最新版本信息
	releaseInfo, err := r.fetchLatestRelease(agentReleaseUrls)
	if err != nil {
		// 根据错误类型返回相应的 HTTP 状态码
		if strings.Contains(err.Error(), "未找到") {
			return utils.ErrorResponse(ctx, http.StatusNotFound, err.Error(), "LATEST_VERSION_NOT_FOUND")
		}
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, err.Error(), err, "FETCH_RELEASE_FAILED")
	}

	// 发送更新命令
	wsService := services.GetWebSocketService()
	message := map[string]interface{}{
		"type":    "command",
		"command": "update",
		"data": map[string]interface{}{
			"version":      releaseInfo.NormalizedTagName,
			"version_type": releaseInfo.VersionType,
		},
	}

	err = wsService.SendMessage(serverID, message)
	if err != nil {
		facades.Log().Errorf("发送更新命令失败: %v", err)
		return utils.ErrorResponseWithError(ctx, http.StatusInternalServerError, "发送更新命令失败", err, "SEND_UPDATE_COMMAND_FAILED")
	}

	facades.Log().Infof("成功发送更新命令到服务器: %s, 版本: %s", serverID, releaseInfo.NormalizedTagName)

	return utils.SuccessResponse(ctx, "更新命令已发送", map[string]interface{}{
		"version":      releaseInfo.NormalizedTagName,
		"version_type": releaseInfo.VersionType,
	})
}

// runMigrations 执行数据库迁移
func (r *UpdateController) runMigrations() error {
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

// restartApplication 重启应用程序
func (r *UpdateController) restartApplication() error {
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

	time.Sleep(500 * time.Millisecond)

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

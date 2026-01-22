package controllers

import (
	"strings"
	"time"

	"goravel/app/services"
	"goravel/app/utils"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type UpdateController struct {
	updateService *services.UpdateService
}

type UpdateStatus struct {
	Step     string `json:"step"`     // connecting, downloading, verifying, unpacking, restarting, completed, error
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

// releaseUrls 版本发布地址
var releaseUrls = "https://api.github.com/repos/YunTower/CloudSentinel/releases/latest"

// agentReleaseUrls Agent 版本发布地址
var agentReleaseUrls = "https://api.github.com/repos/YunTower/CloudSentinel-Agent/releases/latest"

func NewUpdateController() *UpdateController {
	return &UpdateController{
		updateService: services.NewUpdateService(),
	}
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

// CompareVersions 比较版本号，返回 true 表示需要更新
func (r *UpdateController) CompareVersions(currentVersion, latestVersion string) bool {
	return r.updateService.CompareVersions(currentVersion, latestVersion)
}

// ParseVersion 解析版本号字符串
func (r *UpdateController) ParseVersion(version string) (string, string, int) {
	return r.updateService.ParseVersion(version)
}

// FetchLatestRelease 从 GitHub API 获取最新版本信息
func (r *UpdateController) FetchLatestRelease(releaseUrl string) (*services.ReleaseInfo, error) {
	return r.updateService.FetchLatestRelease(releaseUrl)
}

// GetSystemInfo 获取系统信息
func (r *UpdateController) GetSystemInfo() (osType, arch string) {
	return r.updateService.GetSystemInfo()
}

// FindAssetByArchitecture 在 assets 中查找匹配的二进制包
func (r *UpdateController) FindAssetByArchitecture(assets []interface{}, osType, arch string) (string, string) {
	return r.updateService.FindAssetByArchitecture(assets, osType, arch)
}

// FindSHA256Asset 在 assets 中查找匹配的 SHA256 文件
func (r *UpdateController) FindSHA256Asset(assets []interface{}, osType, arch string) (string, string) {
	return r.updateService.FindSHA256Asset(assets, osType, arch)
}

// DownloadFile 下载文件
func (r *UpdateController) DownloadFile(url, filePath string, progressCallback func(int)) error {
	return r.updateService.DownloadFile(url, filePath, progressCallback)
}

// ExtractTarGz 解压 tar.gz 文件
func (r *UpdateController) ExtractTarGz(tarGzPath, destDir string) error {
	return r.updateService.ExtractTarGz(tarGzPath, destDir)
}

// CalculateSHA256 计算文件的 SHA256 值
func (r *UpdateController) CalculateSHA256(filePath string) (string, error) {
	return r.updateService.CalculateSHA256(filePath)
}

// CopyFile 复制文件
func (r *UpdateController) CopyFile(src, dst string) error {
	return r.updateService.CopyFile(src, dst)
}

// ReadSHA256File 读取 SHA256 文件内容
func (r *UpdateController) ReadSHA256File(filePath string) (string, error) {
	return r.updateService.ReadSHA256File(filePath)
}

// CleanupTempFiles 清理更新过程中的临时文件
func (r *UpdateController) CleanupTempFiles(files ...string) {
	r.updateService.CleanupTempFiles(files...)
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

		updateSvc := services.NewUpdateService()
		options := services.UpdateOptions{
			Force:          false,
			SkipMigration:  false,
			StatusCallback: setStatus,
			ReleaseURL:     releaseUrls,
		}

		if err := updateSvc.ExecuteUpdate(options); err != nil {
			setStatus("error", 0, err.Error())
		}

		// 保持完成状态一段时间，以便前端读取
		time.Sleep(1 * time.Minute)
		facades.Cache().Forget("update_status")
	}()

	return utils.SuccessResponse(ctx, "更新任务已启动")
}

// checkVersion 检查版本信息的公共方法
func (r *UpdateController) checkVersion(ctx http.Context, releaseUrl string, includeCurrentVersion bool) http.Response {
	releaseInfo, err := r.FetchLatestRelease(releaseUrl)
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
			// 加载上海时区
			shanghaiLocation, err := time.LoadLocation("Asia/Shanghai")
			if err == nil {
				// 转换为上海时间
				shanghaiTime := parsedTime.In(shanghaiLocation)
				publishTime = shanghaiTime.Format("2006-01-02 15:04:05")
			} else {
				// 如果加载时区失败，使用 UTC 时间
				publishTime = parsedTime.Format("2006-01-02 15:04:05")
			}
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
		_, currentVersionType, _ := r.ParseVersion(currentVersion)
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
	releaseInfo, err := r.updateService.FetchLatestRelease(agentReleaseUrls)
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

// RunMigrations 执行数据库迁移
func (r *UpdateController) RunMigrations() error {
	return r.updateService.RunMigrations()
}

// RestartApplication 重启应用程序
func (r *UpdateController) RestartApplication() error {
	return r.updateService.RestartApplication()
}

// GetDatabasePath 获取数据库文件路径
func (r *UpdateController) GetDatabasePath() (string, error) {
	return r.updateService.GetDatabasePath()
}

// BackupDatabase 备份数据库文件
func (r *UpdateController) BackupDatabase() (string, error) {
	return r.updateService.BackupDatabase()
}

// BackupBinary 备份二进制文件
func (r *UpdateController) BackupBinary() (string, error) {
	return r.updateService.BackupBinary()
}

// RestoreDatabase 恢复数据库文件
func (r *UpdateController) RestoreDatabase(backupPath string) error {
	return r.updateService.RestoreDatabase(backupPath)
}

// RestoreBinary 恢复二进制文件
func (r *UpdateController) RestoreBinary(backupPath string) error {
	return r.updateService.RestoreBinary(backupPath)
}

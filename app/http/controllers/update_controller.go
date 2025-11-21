package controllers

import (
	"encoding/json"
	"fmt"
	"strings"

	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type UpdateController struct{}

type UpdateStatus struct {
	Step     string `json:"step"`     // connecting, downloading, verifying, unpacking, restarting, completed, error
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

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
		var cachedStatus UpdateStatus
		err := facades.Cache().Get("update_status", &cachedStatus)
		if err == nil {
			status = cachedStatus
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status": true,
		"data":   status,
	})
}

// Update 执行更新
func (r *UpdateController) Update(ctx http.Context) http.Response {
	// 检查是否已经在更新中
	if facades.Cache().Has("update_status") {
		var status UpdateStatus
		facades.Cache().Get("update_status", &status)
		if status.Step != "completed" && status.Step != "error" && status.Step != "pending" {
			return ctx.Response().Status(400).Json(http.Json{
				"status":  false,
				"message": "更新已在进行中",
			})
		}
	}

	// 启动异步更新任务
	go func() {
		setStatus := func(step string, progress int, message string) {
			status := UpdateStatus{
				Step:     step,
				Progress: progress,
				Message:  message,
			}
			facades.Cache().Put("update_status", status, 10*time.Minute)
		}

		// TODO： 这里需要实现具体的更新操作
		// 1.选择下载源 gitee / github
		// 2.检查二进制包是否存在
		// 3.下载二进制包
		// 4.执行数据库迁移
		// 5.重启程序

		setStatus("connecting", 0, "正在连接更新服务器...")
		time.Sleep(1 * time.Second)

		// 模拟下载进度
		setStatus("downloading", 0, "正在下载更新包...")
		for i := 0; i <= 100; i += 10 {
			setStatus("downloading", i, "正在下载更新包...")
			time.Sleep(500 * time.Millisecond)
		}

		setStatus("verifying", 100, "正在校验文件完整性...")
		time.Sleep(1 * time.Second)

		setStatus("unpacking", 100, "正在解压并替换文件...")
		time.Sleep(2 * time.Second)

		setStatus("restarting", 100, "正在重启服务...")
		time.Sleep(3 * time.Second)

		setStatus("completed", 100, "更新完成！")
		// 保持完成状态一段时间，以便前端读取
		time.Sleep(1 * time.Minute)
		facades.Cache().Forget("update_status")
	}()

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "更新任务已启动",
	})
}

func (r *UpdateController) Check(ctx http.Context) http.Response {
	releaseUrls := map[string]string{
		"github": "https://api.github.com/repos/YunTower/CloudSentinel/releases/latest",
		"gitee":  "https://gitee.com/api/v5/repos/YunTower/CloudSentinel/releases/latest",
	}

	validator, err := ctx.Request().Validate(map[string]string{
		"type": "required|in:gitee,github",
	})
	if err != nil || validator.Fails() {
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "验证失败",
			"code":    "VALIDATION_ERROR",
			"error":   err.Error(),
			"data":    validator.Errors(),
		})
	}

	requestUrl := releaseUrls[ctx.Request().Input("type")]
	response, requestErr := facades.Http().Get(requestUrl)
	if requestErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "请求最新版本信息失败",
			"code":    "REQUEST_LATEST_VERSION_FAILED",
			"error":   requestErr.Error(),
		})
	}

	responseBody, responseErr := response.Body()
	fmt.Println(responseBody)
	if responseErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "读取最新版本信息失败",
			"code":    "READ_LATEST_VERSION_FAILED",
			"error":   responseErr.Error(),
		})
	}
	if response.Status() == 404 {
		return ctx.Response().Status(404).Json(http.Json{
			"status":  false,
			"message": "未找到最新的版本信息，改天再试试吧",
			"code":    "LATEST_VERSION_NOT_FOUND",
			"error":   "Latest version information not found",
		})
	}

	// 格式化响应体
	var result map[string]any
	err = json.Unmarshal([]byte(responseBody), &result)
	if err != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "解析最新版本信息失败",
			"code":    "PARSE_LATEST_VERSION_FAILED",
			"error":   err.Error(),
		})
	}

	// 获取tagName
	tagName, ok := result["tag_name"].(string)
	if !ok {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "最新版本信息格式错误",
			"code":    "LATEST_VERSION_FORMAT_ERROR",
			"error":   "Invalid latest version information format",
		})
	}

	// 格式化版本号
	if len(tagName) > 0 && tagName[0] == 'v' {
		tagName = tagName[1:]
	}

	// 提取当前版本类型
	currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
	currentVersionParts := strings.Split(currentVersion, "-")
	currentVersionType := "release"
	if len(currentVersionParts) > 1 {
		currentVersionType = currentVersionParts[1]
	}

	// 提取最新版本类型
	versionParts := strings.Split(tagName, "-")
	versionType := "release"
	if len(versionParts) > 1 {
		versionType = versionParts[1]
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"latest_version": "1.0.0",
			//"latest_version":       tagName,
			"latest_version_type":  versionType,
			"current_version":      currentVersion,
			"current_version_type": currentVersionType,
			"publish_time":         result["created_at"],
			"change_log":           result["body"],
		},
	})
}

package controllers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type UpdateController struct{}

func NewUpdateController() *UpdateController {
	return &UpdateController{}
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
			"latest_version":       tagName,
			"latest_version_type":  versionType,
			"current_version":      currentVersion,
			"current_version_type": currentVersionType,
			"publish_time":         result["created_at"],
			"change_log":           result["body"],
		},
	})
}

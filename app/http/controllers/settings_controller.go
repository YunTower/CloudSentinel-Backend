package controllers

import (
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type SettingsController struct {
}

func NewSettingsController() *SettingsController {
	return &SettingsController{}
}

// GetPublicSettings 获取公开的系统设置
func (r *SettingsController) GetPublicSettings(ctx http.Context) http.Response {
	// 查询访客登录相关配置
	var allowGuestLogin string
	var guestPasswordEnabled string
	var panelTitle string

	// 查询是否允许访客登录
	guestLoginErr := facades.DB().Table("system_settings").Where("setting_key", "allow_guest_login").Value("setting_value", &allowGuestLogin)
	if guestLoginErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询访客登录配置失败",
			"code":    "CONFIG_ERROR",
			"error":   guestLoginErr.Error(),
		})
	}

	// 查询是否启用访客密码访问
	guestPasswordErr := facades.DB().Table("system_settings").Where("setting_key", "guest_password_enabled").Value("setting_value", &guestPasswordEnabled)
	if guestPasswordErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询访客密码配置失败",
			"code":    "CONFIG_ERROR",
			"error":   guestPasswordErr.Error(),
		})
	}

	// 查询面板标题
	panelTitleErr := facades.DB().Table("system_settings").Where("setting_key", "panel_title").Value("setting_value", &panelTitle)
	if panelTitleErr != nil {
		// 面板标题查询失败不影响主要功能，使用默认值
		panelTitle = "CloudSentinel 云哨"
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "获取公开配置成功",
		"data": map[string]any{
			"allow_guest_login":      allowGuestLogin == "true",
			"guest_password_enabled": guestPasswordEnabled == "true",
			"panel_title":            panelTitle,
		},
	})
}

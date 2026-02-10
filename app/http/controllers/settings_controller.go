package controllers

import (
	"encoding/json"
	"fmt"
	"goravel/app/repositories"
	"goravel/app/utils"
	"goravel/app/utils/notification"
	"strconv"
	"strings"
	"time"

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
	// 批量获取系统设置
	settings := utils.GetSettings([]string{"allow_guest_login", "guest_password_enabled", "panel_title"})

	allowGuestLogin := settings["allow_guest_login"]
	guestPasswordEnabled := settings["guest_password_enabled"]
	panelTitle := settings["panel_title"]
	if panelTitle == "" {
		panelTitle = "CloudSentinel 云哨"
	}

	return utils.SuccessResponse(ctx, "success", map[string]any{
		"allow_guest_login":      allowGuestLogin == "true",
		"guest_password_enabled": guestPasswordEnabled == "true",
		"panel_title":            panelTitle,
	})
}

func (r *SettingsController) GetPanelSettings(ctx http.Context) http.Response {
	panelTitle := utils.GetSetting("panel_title", "CloudSentinel 云哨")
	logRetentionDays := utils.GetSetting("log_retention_days", "30")

	// 提取当前版本类型
	currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
	currentVersionParts := strings.Split(currentVersion, "-")
	currentVersionType := "release"
	if len(currentVersionParts) > 1 {
		currentVersionType = currentVersionParts[1]
	}

	return utils.SuccessResponse(ctx, "success", map[string]any{
		"panel_title":          panelTitle,
		"log_retention_days":   logRetentionDays,
		"current_version":      currentVersion,
		"current_version_type": currentVersionType,
	})
}

func (r *SettingsController) GetPermissionsSettings(ctx http.Context) http.Response {
	// 批量获取系统设置
	settings := utils.GetSettings([]string{
		"allow_guest_login", "guest_password_enabled", "guest_password_hash",
		"admin_username", "hide_sensitive_info", "session_timeout",
		"max_login_attempts", "lockout_duration", "jwt_expiration",
	})

	allowGuestLogin := utils.GetSetting("allow_guest_login", "false")
	guestPasswordEnabled := utils.GetSetting("guest_password_enabled", "false")
	guestPasswordHash := settings["guest_password_hash"]
	adminUsername := utils.GetSetting("admin_username", "admin")
	hideSensitiveInfo := utils.GetSetting("hide_sensitive_info", "true")
	sessionTimeoutSeconds := utils.GetSetting("session_timeout", "3600")
	maxLoginAttempts := utils.GetSetting("max_login_attempts", "5")
	lockoutDurationSeconds := utils.GetSetting("lockout_duration", "900")
	jwtExpirationSeconds := utils.GetSetting("jwt_expiration", "86400")

	parseInt := func(s string, def int64) int64 {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
		return def
	}

	sessionMinutes := int(parseInt(sessionTimeoutSeconds, 3600) / 60)
	lockoutMinutes := int(parseInt(lockoutDurationSeconds, 900) / 60)
	jwtHours := int(parseInt(jwtExpirationSeconds, 86400) / 3600)

	return utils.SuccessResponse(ctx, "success", map[string]any{
		"allowGuest":        allowGuestLogin == "true",
		"enablePassword":    guestPasswordEnabled == "true",
		"guestPassword":     "",
		"hasPassword":       guestPasswordHash != "",
		"hideSensitiveInfo": hideSensitiveInfo == "true",
		"sessionTimeout":    sessionMinutes,
		"maxLoginAttempts":  parseInt(maxLoginAttempts, 5),
		"lockoutDuration":   lockoutMinutes,
		"jwtExpiration":     jwtHours,
		"adminUsername":     adminUsername,
	})
}

func (r *SettingsController) GetAlertsSettings(ctx http.Context) http.Response {
	notificationRepo := repositories.GetAlertNotificationRepository()

	type notifyConfig struct {
		Enabled bool           `json:"enabled"`
		Config  map[string]any `json:"config"`
	}

	fetchNotify := func(nType string) notifyConfig {
		notification, err := notificationRepo.GetByType(nType)
		if err != nil || notification == nil {
			return notifyConfig{Enabled: false, Config: map[string]any{}}
		}

		cfg := map[string]any{}
		if notification.ConfigJson != "" {
			_ = json.Unmarshal([]byte(notification.ConfigJson), &cfg)
		}
		return notifyConfig{Enabled: notification.Enabled, Config: cfg}
	}

	email := fetchNotify("email")
	webhook := fetchNotify("webhook")

	// 检查密码是否已设置
	hasPassword := false
	if password, ok := email.Config["password"].(string); ok && password != "" {
		hasPassword = true
	}

	emailData := map[string]any{
		"enabled":     email.Enabled,
		"smtp":        email.Config["smtp"],
		"port":        email.Config["port"],
		"security":    email.Config["security"],
		"from":        email.Config["from"],
		"to":          email.Config["to"],
		"hasPassword": hasPassword,
	}
	webhookData := map[string]any{
		"enabled":   webhook.Enabled,
		"webhook":   webhook.Config["webhook"],
		"mentioned": webhook.Config["mentioned"],
		"platform":  webhook.Config["platform"],
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"notifications": map[string]any{
				"email":   emailData,
				"webhook": webhookData,
			},
		},
	})
}

func (r *SettingsController) UpdatePanelSettings(ctx http.Context) http.Response {
	title := ctx.Request().Input("title")
	logRetentionDays := ctx.Request().Input("log_retention_days")

	if title == "" {
		return utils.ErrorResponse(ctx, 422, "缺少标题参数")
	}

	settingRepo := repositories.GetSystemSettingRepository()
	if err := settingRepo.SetValue("panel_title", title); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新失败", err)
	}

	if logRetentionDays != "" {
		// 验证是否为数字
		if _, err := strconv.Atoi(logRetentionDays); err == nil {
			if err := settingRepo.SetValue("log_retention_days", logRetentionDays); err != nil {
				return utils.ErrorResponseWithError(ctx, 500, "更新日志保留天数失败", err)
			}
		}
	}

	return utils.SuccessResponse(ctx, "success")
}

func (r *SettingsController) UpdatePermissionsSettings(ctx http.Context) http.Response {
	type UpdatePermissionsRequest struct {
		AllowGuest        bool   `json:"allowGuest" form:"allowGuest"`
		EnablePassword    bool   `json:"enablePassword" form:"enablePassword"`
		GuestPassword     string `json:"guestPassword" form:"guestPassword"`
		HideSensitiveInfo bool   `json:"hideSensitiveInfo" form:"hideSensitiveInfo"`
		SessionTimeout    int    `json:"sessionTimeout" form:"sessionTimeout"`
		MaxLoginAttempts  int    `json:"maxLoginAttempts" form:"maxLoginAttempts"`
		LockoutDuration   int    `json:"lockoutDuration" form:"lockoutDuration"`
		JwtSecret         string `json:"jwtSecret" form:"jwtSecret"`
		JwtExpiration     int    `json:"jwtExpiration" form:"jwtExpiration"`
		NewUsername       string `json:"newUsername" form:"newUsername"`
		CurrentPassword   string `json:"currentPassword" form:"currentPassword"`
		NewPassword       string `json:"newPassword" form:"newPassword"`
		ConfirmPassword   string `json:"confirmPassword" form:"confirmPassword"`
	}

	var req UpdatePermissionsRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}

	allowGuest := req.AllowGuest
	enablePassword := req.EnablePassword
	guestPassword := req.GuestPassword
	hideSensitiveInfo := req.HideSensitiveInfo
	sessionMinutes := req.SessionTimeout
	maxLoginAttempts := req.MaxLoginAttempts
	lockoutMinutes := req.LockoutDuration
	jwtSecret := req.JwtSecret
	jwtHours := req.JwtExpiration
	newUsername := req.NewUsername
	currentPassword := req.CurrentPassword
	newPassword := req.NewPassword
	confirmPassword := req.ConfirmPassword

	sessionSeconds := sessionMinutes * 60
	lockoutSeconds := lockoutMinutes * 60
	jwtSeconds := jwtHours * 3600

	settingRepo := repositories.GetSystemSettingRepository()
	write := func(key, val, typ string) error {
		return settingRepo.SetValue(key, val)
	}

	if err := write("allow_guest_login", map[bool]string{true: "true", false: "false"}[allowGuest], "boolean"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("guest_password_enabled", map[bool]string{true: "true", false: "false"}[enablePassword], "boolean"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("hide_sensitive_info", map[bool]string{true: "true", false: "false"}[hideSensitiveInfo], "boolean"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("session_timeout", strconv.Itoa(sessionSeconds), "number"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("max_login_attempts", strconv.Itoa(maxLoginAttempts), "number"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("lockout_duration", strconv.Itoa(lockoutSeconds), "number"); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if jwtSecret != "" {
		if err := write("jwt_secret", jwtSecret, "string"); err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
		}
	}
	if jwtSeconds > 0 {
		if err := write("jwt_expiration", strconv.Itoa(jwtSeconds), "number"); err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
		}
	}

	// 处理访客密码hash
	if enablePassword {
		if guestPassword != "" {
			hash, err := facades.Hash().Make(guestPassword)
			if err != nil {
				return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "加密失败", "error": err.Error()})
			}
			if err := write("guest_password_hash", hash, "string"); err != nil {
				return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
			}
		}
	}

	// 处理管理员用户名修改
	if newUsername != "" && currentPassword != "" {
		// 验证当前密码
		userPasswordHash := settingRepo.GetValue("admin_password_hash", "")
		if userPasswordHash == "" {
			return utils.ErrorResponse(ctx, 500, "查询密码配置失败")
		}

		if userPasswordHash == "" {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "密码配置不存在",
			})
		}

		// 验证当前密码
		if !facades.Hash().Check(currentPassword, userPasswordHash) {
			return ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "当前密码错误",
			})
		}

		// 查询当前用户名
		var currentUsername string
		if err := facades.DB().Table("system_settings").Where("setting_key", "admin_username").Value("setting_value", &currentUsername); err != nil {
			currentUsername = ""
		}

		// 检查新用户名是否与当前用户名相同
		if newUsername != currentUsername {
			if err := write("admin_username", newUsername, "string"); err != nil {
				return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新用户名失败", "error": err.Error()})
			}
		}
	}

	// 处理管理员密码修改
	if newPassword != "" && confirmPassword != "" && currentPassword != "" {
		// 验证新密码长度
		if len(newPassword) < 6 {
			return ctx.Response().Status(422).Json(http.Json{
				"status":  false,
				"message": "新密码长度至少为6位",
			})
		}

		// 验证新密码与确认密码是否一致
		if newPassword != confirmPassword {
			return ctx.Response().Status(422).Json(http.Json{
				"status":  false,
				"message": "新密码与确认密码不一致",
			})
		}

		// 验证当前密码
		var userPasswordHash string
		if err := facades.DB().Table("system_settings").Where("setting_key", "admin_password_hash").Value("setting_value", &userPasswordHash); err != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "查询密码配置失败",
				"error":   err.Error(),
			})
		}

		if userPasswordHash == "" {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "密码配置不存在",
			})
		}

		// 验证当前密码
		if !facades.Hash().Check(currentPassword, userPasswordHash) {
			return utils.ErrorResponse(ctx, 401, "当前密码错误")
		}

		// 生成新密码hash
		newPasswordHash, err := facades.Hash().Make(newPassword)
		if err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "密码加密失败", err)
		}

		// 更新密码hash
		if err := write("admin_password_hash", newPasswordHash, "string"); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "更新密码失败", err)
		}
	}

	return utils.SuccessResponse(ctx, "success")
}

func (r *SettingsController) UpdateAlertsSettings(ctx http.Context) http.Response {
	notificationRepo := repositories.GetAlertNotificationRepository()

	emailEnabled := ctx.Request().Input("notifications.email.enabled") == "true"
	emailCfg := map[string]any{
		"smtp":     ctx.Request().Input("notifications.email.smtp"),
		"port":     func() int { v, _ := strconv.Atoi(ctx.Request().Input("notifications.email.port")); return v }(),
		"security": ctx.Request().Input("notifications.email.security"),
		"from":     ctx.Request().Input("notifications.email.from"),
		"to":       ctx.Request().Input("notifications.email.to"),
		"password": ctx.Request().Input("notifications.email.password"),
	}
	webhookEnabled := ctx.Request().Input("notifications.webhook.enabled") == "true"
	webhookCfg := map[string]any{
		"webhook":   ctx.Request().Input("notifications.webhook.webhook"),
		"mentioned": ctx.Request().Input("notifications.webhook.mentioned"),
		"platform":  ctx.Request().Input("notifications.webhook.platform"),
	}
	writeNotify := func(nType string, enabled bool, cfg map[string]any) error {
		// 如果是邮件配置，处理密码逻辑
		if nType == "email" {
			password, _ := cfg["password"].(string)
			// 如果密码为空，尝试读取旧配置中的密码
			if password == "" {
				oldNotification, err := notificationRepo.GetByType("email")
				if err == nil && oldNotification != nil && oldNotification.ConfigJson != "" {
					var oldCfg map[string]any
					if err := json.Unmarshal([]byte(oldNotification.ConfigJson), &oldCfg); err == nil {
						if oldPwd, ok := oldCfg["password"].(string); ok {
							cfg["password"] = oldPwd
						}
					}
				}
			}
		}

		return notificationRepo.UpdateConfig(nType, cfg)
	}
	if err := writeNotify("email", emailEnabled, emailCfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新邮件通知失败", err)
	}
	if err := writeNotify("webhook", webhookEnabled, webhookCfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新Webhook通知失败", err)
	}

	return utils.SuccessResponse(ctx, "success")
}

func (r *SettingsController) TestAlertSettings(ctx http.Context) http.Response {
	channel := ctx.Request().Input("type")
	if channel == "" {
		return utils.ErrorResponse(ctx, 422, "测试类型不能为空")
	}

	notificationRepo := repositories.GetAlertNotificationRepository()

	// 解析配置
	var configJson map[string]interface{}
	if err := ctx.Request().Bind(&configJson); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "无效的请求数据", err)
	}

	configData, ok := configJson["config"].(map[string]interface{})
	if !ok {
		return utils.ErrorResponse(ctx, 422, "无效的配置数据")
	}

	// 序列化配置以便绑定到结构体
	configBytes, err := json.Marshal(configData)
	if err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "配置处理失败", err)
	}

	switch channel {
	case "email":
		var emailCfg notification.EmailConfig
		if err := json.Unmarshal(configBytes, &emailCfg); err != nil {
			return utils.ErrorResponseWithError(ctx, 422, "无效的邮件配置", err)
		}

		// 如果密码为空，尝试使用已保存的密码
		if emailCfg.Password == "" {
			savedNotification, err := notificationRepo.GetByType("email")
			if err == nil && savedNotification != nil && savedNotification.ConfigJson != "" {
				var savedCfg notification.EmailConfig
				if err := json.Unmarshal([]byte(savedNotification.ConfigJson), &savedCfg); err == nil {
					if savedCfg.Password != "" {
						emailCfg.Password = savedCfg.Password
					}
				}
			}
		}

		// 发送测试邮件
		subject := "CloudSentinel 告警通知测试"
		content := fmt.Sprintf("这是一条测试邮件，用于验证您的邮件通知配置是否正确。\n发送时间：%s", time.Now().Format("2006-01-02 15:04:05"))

		if err := notification.SendEmail(emailCfg, subject, content); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "发送测试邮件失败", err)
		}

	case "webhook":
		var webhookCfg notification.WebhookConfig
		if err := json.Unmarshal(configBytes, &webhookCfg); err != nil {
			return utils.ErrorResponseWithError(ctx, 422, "无效的Webhook配置", err)
		}

		// 发送测试消息
		content := fmt.Sprintf("CloudSentinel 告警通知测试\n这是一条测试消息，用于验证您的Webhook通知配置是否正确。\n发送时间：%s", time.Now().Format("2006-01-02 15:04:05"))

		if err := notification.SendWebhook(webhookCfg, content); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "发送测试消息失败", err)
		}

	default:
		return utils.ErrorResponse(ctx, 422, "不支持的通知类型")
	}

	return utils.SuccessResponse(ctx, "测试发送成功")
}

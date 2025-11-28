package controllers

import (
	"encoding/json"
	"fmt"
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

	// 提取当前版本类型
	currentVersion := facades.Config().GetString("app.version", "0.0.1-release")
	currentVersionParts := strings.Split(currentVersion, "-")
	currentVersionType := "release"
	if len(currentVersionParts) > 1 {
		currentVersionType = currentVersionParts[1]
	}

	return utils.SuccessResponse(ctx, "success", map[string]any{
		"panel_title":          panelTitle,
		"current_version":      currentVersion,
		"current_version_type": currentVersionType,
	})
}

func (r *SettingsController) GetPermissionsSettings(ctx http.Context) http.Response {
	// 批量获取系统设置
	settings := utils.GetSettings([]string{
		"allow_guest_login", "guest_password_enabled", "guest_password_hash",
		"admin_username", "hide_sensitive_info", "session_timeout",
		"max_login_attempts", "lockout_duration", "jwt_secret", "jwt_expiration",
	})

	allowGuestLogin := utils.GetSetting("allow_guest_login", "false")
	guestPasswordEnabled := utils.GetSetting("guest_password_enabled", "false")
	guestPasswordHash := settings["guest_password_hash"]
	adminUsername := utils.GetSetting("admin_username", "admin")
	hideSensitiveInfo := utils.GetSetting("hide_sensitive_info", "true")
	sessionTimeoutSeconds := utils.GetSetting("session_timeout", "3600")
	maxLoginAttempts := utils.GetSetting("max_login_attempts", "5")
	lockoutDurationSeconds := utils.GetSetting("lockout_duration", "900")
	jwtSecret := settings["jwt_secret"]
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
		"jwtSecret":         jwtSecret,
		"jwtExpiration":     jwtHours,
		"adminUsername":     adminUsername,
	})
}

func (r *SettingsController) GetAlertsSettings(ctx http.Context) http.Response {
	fetchRule := func(metric string) map[string]any {
		// 默认值
		defaultRule := map[string]any{
			"enabled":  false,
			"warning":  80.0,
			"critical": 90.0,
		}
		if metric == "memory" || metric == "disk" {
			defaultRule["warning"] = 85.0
			defaultRule["critical"] = 95.0
		}

		var ruleJson string
		key := fmt.Sprintf("alert_rule_%s", metric)
		if err := facades.DB().Table("system_settings").Where("setting_key", key).Value("setting_value", &ruleJson); err == nil && ruleJson != "" {
			var rule map[string]any
			if err := json.Unmarshal([]byte(ruleJson), &rule); err == nil {
				// 确保所有字段都存在
				if enabled, ok := rule["enabled"].(bool); ok {
					defaultRule["enabled"] = enabled
				}
				if warning, ok := rule["warning"].(float64); ok {
					defaultRule["warning"] = warning
				} else if warningStr, ok := rule["warning"].(string); ok {
					if w, err := strconv.ParseFloat(warningStr, 64); err == nil {
						defaultRule["warning"] = w
					}
				}
				if critical, ok := rule["critical"].(float64); ok {
					defaultRule["critical"] = critical
				} else if criticalStr, ok := rule["critical"].(string); ok {
					if c, err := strconv.ParseFloat(criticalStr, 64); err == nil {
						defaultRule["critical"] = c
					}
				}
			}
		}

		return defaultRule
	}

	type notifyConfig struct {
		Enabled bool           `json:"enabled"`
		Config  map[string]any `json:"config"`
	}

	fetchNotify := func(nType string) notifyConfig {
		var enabled bool
		var configJson string
		_ = facades.DB().Table("alert_notifications").Where("notification_type", nType).Value("enabled", &enabled)
		_ = facades.DB().Table("alert_notifications").Where("notification_type", nType).Value("config_json", &configJson)

		cfg := map[string]any{}
		if configJson != "" {
			_ = json.Unmarshal([]byte(configJson), &cfg)
		}
		return notifyConfig{Enabled: enabled, Config: cfg}
	}

	cpu := fetchRule("cpu")
	memory := fetchRule("memory")
	disk := fetchRule("disk")

	email := fetchNotify("email")
	webhook := fetchNotify("webhook")
	emailData := map[string]any{
		"enabled":  email.Enabled,
		"smtp":     email.Config["smtp"],
		"port":     email.Config["port"],
		"security": email.Config["security"],
		"from":     email.Config["from"],
		"to":       email.Config["to"],
	}
	webhookData := map[string]any{
		"enabled":   webhook.Enabled,
		"webhook":   webhook.Config["webhook"],
		"mentioned": webhook.Config["mentioned"],
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"rules": map[string]any{
				"cpu":    cpu,
				"memory": memory,
				"disk":   disk,
			},
			"notifications": map[string]any{
				"email":   emailData,
				"webhook": webhookData,
			},
		},
	})
}

func (r *SettingsController) UpdatePanelSettings(ctx http.Context) http.Response {
	title := ctx.Request().Input("title")
	if title == "" {
		return ctx.Response().Status(422).Json(http.Json{
			"status":  false,
			"message": "缺少标题参数",
		})
	}

	now := time.Now().Unix()
	var exists string
	_ = facades.DB().Table("system_settings").Where("setting_key", "panel_title").Value("setting_value", &exists)
	if exists == "" {
		if err := facades.Orm().Query().Table("system_settings").Create(map[string]any{
			"setting_key":   "panel_title",
			"setting_value": title,
			"setting_type":  "string",
			"created_at":    now,
			"updated_at":    now,
		}); err != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "创建失败",
				"error":   err.Error(),
			})
		}
	} else {
		if _, err := facades.Orm().Query().Table("system_settings").Where("setting_key", "panel_title").Update(map[string]any{
			"setting_value": title,
			"updated_at":    now,
		}); err != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "更新失败",
				"error":   err.Error(),
			})
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
	})
}

func (r *SettingsController) UpdatePermissionsSettings(ctx http.Context) http.Response {
	allowGuest := ctx.Request().Input("allowGuest") == "true"
	enablePassword := ctx.Request().Input("enablePassword") == "true"
	guestPassword := ctx.Request().Input("guestPassword")
	hideSensitiveInfo := ctx.Request().Input("hideSensitiveInfo") == "true"

	sessionMinutes, _ := strconv.Atoi(ctx.Request().Input("sessionTimeout"))
	maxLoginAttempts, _ := strconv.Atoi(ctx.Request().Input("maxLoginAttempts"))
	lockoutMinutes, _ := strconv.Atoi(ctx.Request().Input("lockoutDuration"))
	jwtSecret := ctx.Request().Input("jwtSecret")
	jwtHours, _ := strconv.Atoi(ctx.Request().Input("jwtExpiration"))

	newUsername := ctx.Request().Input("newUsername")
	currentPassword := ctx.Request().Input("currentPassword")
	newPassword := ctx.Request().Input("newPassword")
	confirmPassword := ctx.Request().Input("confirmPassword")

	sessionSeconds := sessionMinutes * 60
	lockoutSeconds := lockoutMinutes * 60
	jwtSeconds := jwtHours * 3600

	now := time.Now().Unix()
	write := func(key, val, typ string) error {
		var exists string
		_ = facades.DB().Table("system_settings").Where("setting_key", key).Value("setting_value", &exists)
		if exists == "" {
			if err := facades.Orm().Query().Table("system_settings").Create(map[string]any{
				"setting_key":   key,
				"setting_value": val,
				"setting_type":  typ,
				"created_at":    now,
				"updated_at":    now,
			}); err != nil {
				return err
			}
			return nil
		}
		if _, err := facades.Orm().Query().Table("system_settings").Where("setting_key", key).Update(map[string]any{
			"setting_value": val,
			"updated_at":    now,
		}); err != nil {
			return err
		}
		return nil
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
			return ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "当前密码错误",
			})
		}

		// 生成新密码hash
		newPasswordHash, err := facades.Hash().Make(newPassword)
		if err != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "密码加密失败",
				"error":   err.Error(),
			})
		}

		// 更新密码hash
		if err := write("admin_password_hash", newPasswordHash, "string"); err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新密码失败", "error": err.Error()})
		}
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
	})
}

func (r *SettingsController) UpdateAlertsSettings(ctx http.Context) http.Response {
	now := time.Now().Unix()

	type rule struct {
		Enabled  bool
		Warning  float64
		Critical float64
	}
	readRule := func(metric string) rule {
		enabled := ctx.Request().Input("rules."+metric+".enabled") == "true"
		warning, _ := strconv.ParseFloat(ctx.Request().Input("rules."+metric+".warning"), 64)
		critical, _ := strconv.ParseFloat(ctx.Request().Input("rules."+metric+".critical"), 64)
		return rule{Enabled: enabled, Warning: warning, Critical: critical}
	}
	writeRule := func(metric string, rl rule) error {
		// 将规则序列化为 JSON
		ruleData := map[string]any{
			"enabled":  rl.Enabled,
			"warning":  rl.Warning,
			"critical": rl.Critical,
		}
		ruleJson, err := json.Marshal(ruleData)
		if err != nil {
			return err
		}

		// 保存到 system_settings 表
		key := fmt.Sprintf("alert_rule_%s", metric)
		var exists string
		_ = facades.DB().Table("system_settings").Where("setting_key", key).Value("setting_value", &exists)
		if exists == "" {
			if err := facades.Orm().Query().Table("system_settings").Create(map[string]any{
				"setting_key":   key,
				"setting_value": string(ruleJson),
				"setting_type":  "json",
				"created_at":    now,
				"updated_at":    now,
			}); err != nil {
				return err
			}
			return nil
		}
		if _, err := facades.Orm().Query().Table("system_settings").Where("setting_key", key).Update(map[string]any{
			"setting_value": string(ruleJson),
			"updated_at":    now,
		}); err != nil {
			return err
		}
		return nil
	}
	if err := writeRule("cpu", readRule("cpu")); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新CPU规则失败", "error": err.Error()})
	}
	if err := writeRule("memory", readRule("memory")); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新内存规则失败", "error": err.Error()})
	}
	if err := writeRule("disk", readRule("disk")); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新磁盘规则失败", "error": err.Error()})
	}

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
	}
	writeNotify := func(nType string, enabled bool, cfg map[string]any) error {
		// 如果是邮件配置，处理密码逻辑
		if nType == "email" {
			password, _ := cfg["password"].(string)
			// 如果密码为空，尝试读取旧配置中的密码
			if password == "" {
				var oldConfigJson string
				_ = facades.DB().Table("alert_notifications").Where("notification_type", "email").Value("config_json", &oldConfigJson)
				if oldConfigJson != "" {
					var oldCfg map[string]any
					if err := json.Unmarshal([]byte(oldConfigJson), &oldCfg); err == nil {
						if oldPwd, ok := oldCfg["password"].(string); ok {
							cfg["password"] = oldPwd
						}
					}
				}
			}
		}

		cfgBytes, _ := json.Marshal(cfg)
		var exists string
		// 先检查是否存在该类型的配置
		_ = facades.DB().Table("alert_notifications").Where("notification_type", nType).Value("notification_type", &exists)

		data := map[string]any{
			"notification_type": nType,
			"enabled":           enabled,
			"config_json":       string(cfgBytes),
			"updated_at":        now,
		}
		if exists == "" {
			data["created_at"] = now
			if err := facades.Orm().Query().Table("alert_notifications").Create(data); err != nil {
				return err
			}
			return nil
		}
		if _, err := facades.Orm().Query().Table("alert_notifications").Where("notification_type", nType).Update(data); err != nil {
			return err
		}
		return nil
	}
	if err := writeNotify("email", emailEnabled, emailCfg); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新邮件通知失败", "error": err.Error()})
	}
	if err := writeNotify("webhook", webhookEnabled, webhookCfg); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新Webhook通知失败", "error": err.Error()})
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
	})
}

func (r *SettingsController) TestAlertSettings(ctx http.Context) http.Response {
	channel := ctx.Request().Input("type")
	if channel == "" {
		return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "测试类型不能为空"})
	}

	// 解析配置
	var configJson map[string]interface{}
	if err := ctx.Request().Bind(&configJson); err != nil {
		return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "无效的请求数据", "error": err.Error()})
	}

	configData, ok := configJson["config"].(map[string]interface{})
	if !ok {
		return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "无效的配置数据"})
	}

	// 序列化配置以便绑定到结构体
	configBytes, err := json.Marshal(configData)
	if err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "配置处理失败", "error": err.Error()})
	}

	switch channel {
	case "email":
		var emailCfg notification.EmailConfig
		if err := json.Unmarshal(configBytes, &emailCfg); err != nil {
			return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "无效的邮件配置", "error": err.Error()})
		}

		// 如果密码为空，尝试使用已保存的密码
		if emailCfg.Password == "" {
			var savedConfigJson string
			_ = facades.DB().Table("alert_notifications").Where("notification_type", "email").Value("config_json", &savedConfigJson)
			if savedConfigJson != "" {
				var savedCfg notification.EmailConfig
				if err := json.Unmarshal([]byte(savedConfigJson), &savedCfg); err == nil {
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
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "发送测试邮件失败: " + err.Error()})
		}

	case "webhook":
		var webhookCfg notification.WebhookConfig
		if err := json.Unmarshal(configBytes, &webhookCfg); err != nil {
			return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "无效的Webhook配置", "error": err.Error()})
		}

		// 发送测试消息
		content := fmt.Sprintf("CloudSentinel 告警通知测试\n这是一条测试消息，用于验证您的Webhook通知配置是否正确。\n发送时间：%s", time.Now().Format("2006-01-02 15:04:05"))

		if err := notification.SendWebhook(webhookCfg, content); err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "发送测试消息失败: " + err.Error()})
		}

	default:
		return ctx.Response().Status(422).Json(http.Json{"status": false, "message": "不支持的通知类型"})
	}

	return ctx.Response().Success().Json(http.Json{"status": true, "message": "测试发送成功"})
}

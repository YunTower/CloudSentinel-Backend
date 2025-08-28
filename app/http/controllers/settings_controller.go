package controllers

import (
	"encoding/json"
	"strconv"
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
		"message": "success",
		"data": map[string]any{
			"allow_guest_login":      allowGuestLogin == "true",
			"guest_password_enabled": guestPasswordEnabled == "true",
			"panel_title":            panelTitle,
		},
	})
}

func (r *SettingsController) GetPanelSettings(ctx http.Context) http.Response {
	var panelTitle string

	panelTitleErr := facades.DB().Table("system_settings").Where("setting_key", "panel_title").Value("setting_value", &panelTitle)
	if panelTitleErr != nil {
		// 面板标题查询失败不影响主要功能，使用默认值
		panelTitle = "CloudSentinel 云哨"
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"panel_title":     panelTitle,
			"current_version": "0.0.1",
		},
	})
}

func (r *SettingsController) GetPermissionsSettings(ctx http.Context) http.Response {
	var allowGuestLogin string
	var guestPasswordEnabled string
	var hideSensitiveInfo string
	var sessionTimeoutSeconds string
	var maxLoginAttempts string
	var lockoutDurationSeconds string
	var jwtSecret string
	var jwtExpirationSeconds string

	if err := facades.DB().Table("system_settings").Where("setting_key", "allow_guest_login").Value("setting_value", &allowGuestLogin); err != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询配置失败",
			"code":    "CONFIG_ERROR",
			"error":   err.Error(),
		})
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "guest_password_enabled").Value("setting_value", &guestPasswordEnabled); err != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询配置失败",
			"code":    "CONFIG_ERROR",
			"error":   err.Error(),
		})
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "hide_sensitive_info").Value("setting_value", &hideSensitiveInfo); err != nil {
		hideSensitiveInfo = "true"
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "session_timeout").Value("setting_value", &sessionTimeoutSeconds); err != nil {
		sessionTimeoutSeconds = "3600"
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "max_login_attempts").Value("setting_value", &maxLoginAttempts); err != nil {
		maxLoginAttempts = "5"
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "lockout_duration").Value("setting_value", &lockoutDurationSeconds); err != nil {
		lockoutDurationSeconds = "900"
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "jwt_secret").Value("setting_value", &jwtSecret); err != nil {
		jwtSecret = ""
	}
	if err := facades.DB().Table("system_settings").Where("setting_key", "jwt_expiration").Value("setting_value", &jwtExpirationSeconds); err != nil {
		jwtExpirationSeconds = "86400"
	}

	parseInt := func(s string, def int64) int64 {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
		return def
	}

	sessionMinutes := int(parseInt(sessionTimeoutSeconds, 3600) / 60)
	lockoutMinutes := int(parseInt(lockoutDurationSeconds, 900) / 60)
	jwtHours := int(parseInt(jwtExpirationSeconds, 86400) / 3600)

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"allowGuest":        allowGuestLogin == "true",
			"enablePassword":    guestPasswordEnabled == "true",
			"guestPassword":     "",
			"hideSensitiveInfo": hideSensitiveInfo == "true",
			"sessionTimeout":    sessionMinutes,
			"maxLoginAttempts":  parseInt(maxLoginAttempts, 5),
			"lockoutDuration":   lockoutMinutes,
			"jwtSecret":         jwtSecret,
			"jwtExpiration":     jwtHours,
		},
	})
}

func (r *SettingsController) GetAlertsSettings(ctx http.Context) http.Response {
	fetchRule := func(metric string) map[string]any {
		var enabled bool
		var warning string
		var critical string

		_ = facades.DB().Table("alert_rules").Where("metric_type", metric).Value("enabled", &enabled)
		_ = facades.DB().Table("alert_rules").Where("metric_type", metric).Value("warning_threshold", &warning)
		_ = facades.DB().Table("alert_rules").Where("metric_type", metric).Value("critical_threshold", &critical)

		warnF, _ := strconv.ParseFloat(warning, 64)
		critF, _ := strconv.ParseFloat(critical, 64)
		return map[string]any{
			"enabled":  enabled,
			"warning":  warnF,
			"critical": critF,
		}
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
	wechat := fetchNotify("wechat")

	emailData := map[string]any{
		"enabled":  email.Enabled,
		"smtp":     email.Config["smtp"],
		"port":     email.Config["port"],
		"security": email.Config["security"],
		"from":     email.Config["from"],
		"to":       email.Config["to"],
	}
	wechatData := map[string]any{
		"enabled":   wechat.Enabled,
		"webhook":   wechat.Config["webhook"],
		"mentioned": wechat.Config["mentioned"],
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
				"email":  emailData,
				"wechat": wechatData,
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

	if enablePassword && guestPassword != "" {
		hash, err := facades.Hash().Make(guestPassword)
		if err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "加密失败", "error": err.Error()})
		}
		if err := write("guest_password_hash", hash, "string"); err != nil {
			return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
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
		var exists string
		_ = facades.DB().Table("alert_rules").Where("metric_type", metric).Value("rule_name", &exists)
		data := map[string]any{
			"rule_name":          metric + "告警",
			"metric_type":        metric,
			"warning_threshold":  rl.Warning,
			"critical_threshold": rl.Critical,
			"enabled":            rl.Enabled,
			"updated_at":         now,
		}
		if exists == "" {
			data["created_at"] = now
			if err := facades.Orm().Query().Table("alert_rules").Create(data); err != nil {
				return err
			}
			return nil
		}
		if _, err := facades.Orm().Query().Table("alert_rules").Where("metric_type", metric).Update(data); err != nil {
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
	}
	wechatEnabled := ctx.Request().Input("notifications.wechat.enabled") == "true"
	wechatCfg := map[string]any{
		"webhook":   ctx.Request().Input("notifications.wechat.webhook"),
		"mentioned": ctx.Request().Input("notifications.wechat.mentioned"),
	}
	writeNotify := func(nType string, enabled bool, cfg map[string]any) error {
		cfgBytes, _ := json.Marshal(cfg)
		var exists string
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
	if err := writeNotify("wechat", wechatEnabled, wechatCfg); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新微信通知失败", "error": err.Error()})
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
	})
}

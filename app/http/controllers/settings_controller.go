package controllers

import (
	"encoding/json"
	"fmt"
	"goravel/app/repositories"
	"goravel/app/services"
	"goravel/app/utils"
	"goravel/app/utils/notification"
	"goravel/app/utils/secret"
	"goravel/app/utils/security"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	"goravel/app/facades"
)

type SettingsController struct {
}

func NewSettingsController() *SettingsController {
	return &SettingsController{}
}

// GetPublicSettings 获取公开设置。
// 未传 path 时仅返回面板标题；传入 path 时校验公开页并只返回该页渲染所需配置。
func (r *SettingsController) GetPublicSettings(ctx http.Context) http.Response {
	path := strings.TrimSpace(ctx.Request().Query("path", ""))
	if path == "" {
		panelTitle := utils.GetSetting("panel_title", "CloudSentinel 云哨")
		if panelTitle == "" {
			panelTitle = "CloudSentinel 云哨"
		}
		return utils.SuccessResponse(ctx, "success", publicSettingsBasePayload(panelTitle))
	}

	resolution, err := getPublicPagePolicy().Resolve(path)
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusNotFound, err.Error())
	}

	publicDisplayCfg := loadPublicDisplayConfigV1()
	return utils.SuccessResponse(ctx, "success", publicPageSettingsPayload(
		publicDisplayCfg,
		resolution.Config.RefreshIntervalSeconds,
		resolution.Page,
	))
}

func publicSettingsBasePayload(panelTitle string) map[string]any {
	return map[string]any{
		"panel_title": panelTitle,
	}
}

func publicPageSettingsPayload(
	publicDisplayCfg PublicDisplayConfigV1,
	refreshIntervalSeconds int,
	page *PublicPageV1,
) map[string]any {
	return map[string]any{
		"public_display": publicPageDisplayPayloadV1(publicDisplayCfg),
		"public_pages": map[string]any{
			"refreshIntervalSeconds": refreshIntervalSeconds,
			"page":                   page,
		},
	}
}

func (r *SettingsController) GetPanelSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	panelTitle := utils.GetSetting("panel_title", "CloudSentinel 云哨")
	logRetentionDays := utils.GetSetting("log_retention_days", "30")
	updateChannel := utils.GetSetting("update_channel", "release")
	if updateChannel != "dev" && updateChannel != "beta" && updateChannel != "release" {
		updateChannel = "release"
	}

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
		"update_channel":       updateChannel,
		"current_version":      currentVersion,
		"current_version_type": currentVersionType,
	})
}

func (r *SettingsController) GetPermissionsSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	adminUsername := utils.GetSetting("admin_username", "admin")
	maxLoginAttempts := utils.GetSetting("max_login_attempts", "5")
	lockoutDurationSeconds := utils.GetSetting("lockout_duration", "900")

	parseInt := func(s string, def int64) int64 {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
		return def
	}

	lockoutMinutes := int(parseInt(lockoutDurationSeconds, 900) / 60)

	// 认证配置动态化（P2-01）：返回 DB 中的会话/JWT 有效期；密钥仅以掩码标示是否显式配置，
	// 不回显任何明文或密文密钥。seeder 占位密钥视为未配置（运行时回退环境变量）。
	sessionTimeoutSec := parseInt(utils.GetSetting("session_timeout", ""), 0)
	jwtExpirationSec := parseInt(utils.GetSetting("jwt_expiration", ""), 0)
	jwtSecret := ""
	if s := utils.GetSetting("jwt_secret", ""); s != "" && s != "cloudsentinel-secret-key-change-in-production" {
		jwtSecret = "***"
	}

	return utils.SuccessResponse(ctx, "success", map[string]any{
		"maxLoginAttempts": parseInt(maxLoginAttempts, 5),
		"lockoutDuration":  lockoutMinutes,
		"adminUsername":    adminUsername,
		"sessionTimeout":   sessionTimeoutSec,
		"jwtExpiration":    jwtExpirationSec,
		"jwtSecret":        jwtSecret,
	})
}

func (r *SettingsController) GetAlertsSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

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

	// isEmailChannelConfigured 邮件渠道已启用且具备最小有效配置
	isEmailChannelConfigured := func(email notifyConfig) bool {
		if !email.Enabled {
			return false
		}
		smtp, _ := email.Config["smtp"].(string)
		from, _ := email.Config["from"].(string)
		to, _ := email.Config["to"].(string)
		return strings.TrimSpace(smtp) != "" && strings.TrimSpace(from) != "" && strings.TrimSpace(to) != ""
	}

	// isWebhookChannelConfigured Webhook 渠道已启用且具备最小有效配置
	isWebhookChannelConfigured := func(wh notifyConfig) bool {
		if !wh.Enabled {
			return false
		}
		rawURL, _ := wh.Config["webhook"].(string)
		url, err := secret.DecryptStringWithAppKey(rawURL)
		return err == nil && strings.TrimSpace(url) != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://"))
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
	rawWebhookURL, _ := webhook.Config["webhook"].(string)
	hasWebhook := strings.TrimSpace(rawWebhookURL) != ""
	// 不回显密文或明文 URL：已配置时以掩码占位，前端“留空即保持原值”
	webhookDisplay := ""
	if hasWebhook {
		webhookDisplay = "***"
	}
	webhookData := map[string]any{
		"enabled":      webhook.Enabled,
		"webhook":      webhookDisplay,
		"hasWebhook":   hasWebhook,
		"clearWebhook": false,
		"mentioned":    webhook.Config["mentioned"],
		"platform":     webhook.Config["platform"],
	}

	// 是否已配置任意一个通知渠道
	hasNotificationChannel := isEmailChannelConfigured(email) || isWebhookChannelConfigured(webhook)

	// 服务器离线/上线告警开关
	settingRepo := repositories.GetSystemSettingRepository()
	alertServerOfflineEnabled := settingRepo.GetBool("alert_server_offline_enabled", false)
	alertServerOnlineEnabled := settingRepo.GetBool("alert_server_online_enabled", false)

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "success",
		"data": map[string]any{
			"notifications": map[string]any{
				"email":   emailData,
				"webhook": webhookData,
			},
			"hasNotificationChannel":    hasNotificationChannel,
			"alertServerOfflineEnabled": alertServerOfflineEnabled,
			"alertServerOnlineEnabled":  alertServerOnlineEnabled,
		},
	})
}

func (r *SettingsController) UpdatePanelSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	var req struct {
		Title            string `json:"title" form:"title"`
		LogRetentionDays *int   `json:"log_retention_days" form:"log_retention_days"`
		UpdateChannel    string `json:"update_channel" form:"update_channel"`
	}
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}

	// 从 All() 取 update_channel，避免 PATCH JSON 只被部分解析时拿不到（Goravel：All 为 json+form+query 合集）
	allData := ctx.Request().All()
	if u, ok := allData["update_channel"]; ok && req.UpdateChannel == "" {
		if s, ok := u.(string); ok && s != "" {
			req.UpdateChannel = s
		}
	}

	if req.Title == "" {
		return utils.ErrorResponse(ctx, 422, "缺少标题参数")
	}

	settingRepo := repositories.GetSystemSettingRepository()
	if err := settingRepo.SetValue("panel_title", req.Title); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新失败", err)
	}

	if req.LogRetentionDays != nil {
		if *req.LogRetentionDays >= 1 && *req.LogRetentionDays <= 365 {
			if err := settingRepo.SetValue("log_retention_days", strconv.Itoa(*req.LogRetentionDays)); err != nil {
				return utils.ErrorResponseWithError(ctx, 500, "更新日志保留天数失败", err)
			}
		}
	}

	// 更新渠道：请求有则校验并写入，无则用库中或默认 release；SetValue 内部会无则新建
	updateChannel := req.UpdateChannel
	if updateChannel != "" {
		if updateChannel != "dev" && updateChannel != "beta" && updateChannel != "release" {
			return utils.ErrorResponse(ctx, 422, "更新渠道只能是 dev、beta 或 release")
		}
	} else {
		updateChannel = settingRepo.GetValue("update_channel", "release")
	}
	if err := settingRepo.SetValue("update_channel", updateChannel); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新渠道保存失败", err)
	}

	return utils.SuccessResponse(ctx, "success")
}

func (r *SettingsController) UpdatePermissionsSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	type UpdatePermissionsRequest struct {
		MaxLoginAttempts int    `json:"maxLoginAttempts" form:"maxLoginAttempts"`
		LockoutDuration  int    `json:"lockoutDuration" form:"lockoutDuration"`
		SessionTimeout   *int   `json:"sessionTimeout" form:"sessionTimeout"`
		JwtExpiration    *int   `json:"jwtExpiration" form:"jwtExpiration"`
		JwtSecret        string `json:"jwtSecret" form:"jwtSecret"`
		NewUsername      string `json:"newUsername" form:"newUsername"`
		CurrentPassword  string `json:"currentPassword" form:"currentPassword"`
		NewPassword      string `json:"newPassword" form:"newPassword"`
		ConfirmPassword  string `json:"confirmPassword" form:"confirmPassword"`
	}

	var req UpdatePermissionsRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}

	maxLoginAttempts := req.MaxLoginAttempts
	lockoutMinutes := req.LockoutDuration
	newUsername := req.NewUsername
	currentPassword := req.CurrentPassword
	newPassword := req.NewPassword
	confirmPassword := req.ConfirmPassword

	lockoutSeconds := lockoutMinutes * 60

	settingRepo := repositories.GetSystemSettingRepository()
	write := func(key, val string) error {
		return settingRepo.SetValue(key, val)
	}

	if err := write("max_login_attempts", strconv.Itoa(maxLoginAttempts)); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}
	if err := write("lockout_duration", strconv.Itoa(lockoutSeconds)); err != nil {
		return ctx.Response().Status(500).Json(http.Json{"status": false, "message": "更新失败", "error": err.Error()})
	}

	// 认证配置动态化（P2-01）：会话/JWT 有效期（秒）仅 >0 时写入，留空不更改
	if req.SessionTimeout != nil && *req.SessionTimeout > 0 {
		if err := write("session_timeout", strconv.Itoa(*req.SessionTimeout)); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "更新会话有效期失败", err)
		}
	}
	if req.JwtExpiration != nil && *req.JwtExpiration > 0 {
		if err := write("jwt_expiration", strconv.Itoa(*req.JwtExpiration)); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "更新JWT有效期失败", err)
		}
	}

	// JWT 密钥：留空不更改；拒绝掩码占位与过短值；密钥本身不回显
	jwtSecret := strings.TrimSpace(req.JwtSecret)
	if jwtSecret != "" && jwtSecret != "***" {
		if len(jwtSecret) < 16 {
			return utils.ErrorResponse(ctx, 422, "JWT 密钥至少 16 位")
		}
		if err := write("jwt_secret", jwtSecret); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "更新JWT密钥失败", err)
		}
	}

	// 认证配置已变更：同步到运行时。轮换密钥后旧 token 立即失效，需重新登录。
	services.SyncAuthSettingsFromDB()

	// 处理管理员用户名修改
	if newUsername != "" && currentPassword != "" {
		// 验证当前密码
		userPasswordHash := settingRepo.GetValue("admin_password_hash", "")
		if userPasswordHash == "" {
			return utils.ErrorResponse(ctx, 500, "查询密码配置失败")
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
			if err := write("admin_username", newUsername); err != nil {
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
		if err := write("admin_password_hash", newPasswordHash); err != nil {
			return utils.ErrorResponseWithError(ctx, 500, "更新密码失败", err)
		}
	}

	return utils.SuccessResponse(ctx, "success")
}

func (r *SettingsController) UpdateAlertsSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	notificationRepo := repositories.GetAlertNotificationRepository()
	settingRepo := repositories.GetSystemSettingRepository()

	type alertsEmailReq struct {
		Enabled  bool   `json:"enabled"`
		SMTP     string `json:"smtp"`
		Port     int    `json:"port"`
		Security string `json:"security"`
		From     string `json:"from"`
		To       string `json:"to"`
		Password string `json:"password"`
	}
	type alertsWebhookReq struct {
		Enabled      bool   `json:"enabled"`
		Webhook      string `json:"webhook"`
		ClearWebhook bool   `json:"clearWebhook"`
		Mentioned    string `json:"mentioned"`
		Platform     string `json:"platform"`
	}
	type UpdateAlertsRequest struct {
		Notifications struct {
			Email   alertsEmailReq   `json:"email"`
			Webhook alertsWebhookReq `json:"webhook"`
		} `json:"notifications"`
		AlertServerOfflineEnabled bool `json:"alertServerOfflineEnabled"`
		AlertServerOnlineEnabled  bool `json:"alertServerOnlineEnabled"`
	}

	var req UpdateAlertsRequest
	if err := ctx.Request().Bind(&req); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}

	emailEnabled := req.Notifications.Email.Enabled
	emailCfg := map[string]any{
		"smtp":     req.Notifications.Email.SMTP,
		"port":     req.Notifications.Email.Port,
		"security": req.Notifications.Email.Security,
		"from":     req.Notifications.Email.From,
		"to":       req.Notifications.Email.To,
		"password": req.Notifications.Email.Password,
	}
	webhookEnabled := req.Notifications.Webhook.Enabled
	clearWebhook := req.Notifications.Webhook.ClearWebhook
	webhookCfg := map[string]any{
		"webhook":   req.Notifications.Webhook.Webhook,
		"mentioned": req.Notifications.Webhook.Mentioned,
		"platform":  req.Notifications.Webhook.Platform,
	}

	webhookURL := strings.TrimSpace(fmt.Sprint(webhookCfg["webhook"]))
	if clearWebhook {
		webhookURL = ""
		webhookCfg["webhook"] = ""
	} else if webhookURL == "" {
		oldNotification, err := notificationRepo.GetByType("webhook")
		if err == nil && oldNotification != nil && oldNotification.ConfigJson != "" {
			var oldCfg map[string]any
			if err := json.Unmarshal([]byte(oldNotification.ConfigJson), &oldCfg); err == nil {
				if oldURL, ok := oldCfg["webhook"].(string); ok && oldURL != "" {
					webhookCfg["webhook"] = oldURL
					if decryptedURL, err := secret.DecryptStringWithAppKey(oldURL); err == nil {
						webhookURL = strings.TrimSpace(decryptedURL)
					}
				}
			}
		}
	} else {
		u, err := security.ParseAndValidateWebhookURLForConfig(webhookURL)
		if err != nil {
			return utils.ErrorResponse(ctx, 422, "Webhook URL 不合法")
		}
		webhookURL = u.String()
		webhookCfg["webhook"] = webhookURL
	}
	if webhookEnabled && webhookURL == "" {
		return utils.ErrorResponse(ctx, 422, "启用 Webhook 通知时必须填写有效的 Webhook URL")
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

		return notificationRepo.UpdateConfig(nType, enabled, cfg)
	}
	if err := writeNotify("email", emailEnabled, emailCfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新邮件通知失败", err)
	}
	if err := writeNotify("webhook", webhookEnabled, webhookCfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新Webhook通知失败", err)
	}

	// 服务器离线/上线告警开关（已通过 Bind 读取）
	alertServerOfflineEnabled := req.AlertServerOfflineEnabled
	alertServerOnlineEnabled := req.AlertServerOnlineEnabled

	emailConfigured := emailEnabled && strings.TrimSpace(fmt.Sprint(emailCfg["smtp"])) != "" &&
		strings.TrimSpace(fmt.Sprint(emailCfg["from"])) != "" && strings.TrimSpace(fmt.Sprint(emailCfg["to"])) != ""
	webhookConfigured := webhookEnabled && webhookURL != ""
	hasChannel := emailConfigured || webhookConfigured

	if (alertServerOfflineEnabled || alertServerOnlineEnabled) && !hasChannel {
		return utils.ErrorResponse(ctx, 422, "请先配置并启用至少一个通知渠道（邮件或 Webhook）后再开启服务器离线/上线告警")
	}

	if err := settingRepo.SetValue("alert_server_offline_enabled", map[bool]string{true: "true", false: "false"}[alertServerOfflineEnabled]); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新服务器离线告警设置失败", err)
	}
	if err := settingRepo.SetValue("alert_server_online_enabled", map[bool]string{true: "true", false: "false"}[alertServerOnlineEnabled]); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "更新服务器上线告警设置失败", err)
	}

	return utils.SuccessResponse(ctx, "success")
}

// GetPublicDisplaySettings 管理员读取公开展示配置（V1）
func (r *SettingsController) GetPublicDisplaySettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	cfg := loadPublicDisplayConfigV1()
	return utils.SuccessResponse(ctx, "success", cfg)
}

// UpdatePublicDisplaySettings 管理员更新公开展示配置（V1）
func (r *SettingsController) UpdatePublicDisplaySettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	// 兼容 JSON/form：统一读 All() 再转结构体
	all := ctx.Request().All()
	if len(all) == 0 {
		return utils.ErrorResponse(ctx, 422, "请求数据为空")
	}

	cfg, err := decodePublicDisplayConfigFromAny(all)
	if err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if err := validatePublicDisplayConfigV1(&cfg); err != nil {
		return utils.ErrorResponse(ctx, 422, err.Error())
	}

	if err := savePublicDisplayConfigV1(cfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "保存失败", err)
	}
	return utils.SuccessResponse(ctx, "success", loadPublicDisplayConfigV1())
}

// GetPublicPagesSettings 管理员读取公开页面配置（V1）
func (r *SettingsController) GetPublicPagesSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}
	cfg := loadPublicPagesConfigV1()
	return utils.SuccessResponse(ctx, "success", cfg)
}

// UpdatePublicPagesSettings 管理员更新公开页面配置（V1）
func (r *SettingsController) UpdatePublicPagesSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	// 统一走 All()+JSON，避免 Bind 对嵌套 block.data(RawMessage) 解析丢失
	all := ctx.Request().All()
	if len(all) == 0 {
		return utils.ErrorResponse(ctx, 422, "请求数据为空")
	}
	raw, err := json.Marshal(all)
	if err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}
	var cfg PublicPagesConfigV1
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 422, "请求参数错误", err)
	}
	policy := getPublicPagePolicy()
	if err := policy.NormalizeValidate(&cfg); err != nil {
		return utils.ErrorResponse(ctx, 422, err.Error())
	}
	if err := policy.Save(cfg); err != nil {
		return utils.ErrorResponseWithError(ctx, 500, "保存失败", err)
	}
	return utils.SuccessResponse(ctx, "success", loadPublicPagesConfigV1())
}

func (r *SettingsController) TestAlertSettings(ctx http.Context) http.Response {
	if resp := requireAdmin(ctx); resp != nil {
		return resp
	}

	channel := ctx.Request().Input("type")
	if channel == "" {
		return utils.ErrorResponse(ctx, 422, "测试类型不能为空")
	}

	userID, _ := ctx.Value("user_id").(string)
	facades.Log().Infof("alert test requested: user_id=%s type=%s", userID, channel)

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
						if password, err := secret.DecryptStringWithAppKey(savedCfg.Password); err == nil {
							emailCfg.Password = password
						}
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
		if strings.TrimSpace(webhookCfg.Webhook) == "" {
			savedNotification, err := notificationRepo.GetByType("webhook")
			if err == nil && savedNotification != nil && savedNotification.ConfigJson != "" {
				var savedCfg notification.WebhookConfig
				if err := json.Unmarshal([]byte(savedNotification.ConfigJson), &savedCfg); err == nil && savedCfg.Webhook != "" {
					if webhookURL, err := secret.DecryptStringWithAppKey(savedCfg.Webhook); err == nil {
						webhookCfg.Webhook = webhookURL
					}
				}
			}
		}
		if u, err := security.ParseAndValidateWebhookURLForConfig(webhookCfg.Webhook); err == nil {
			facades.Log().Infof("alert test webhook target: user_id=%s host=%s", userID, u.Hostname())
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

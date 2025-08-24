package seeders

import (
	"time"

	"github.com/goravel/framework/facades"
)

type SystemSettingsSeeder struct {
}

// Signature The name and signature of the seeder.
func (s *SystemSettingsSeeder) Signature() string {
	return "SystemSettingsSeeder"
}

// Run executes the seeder logic.
func (s *SystemSettingsSeeder) Run() error {
	// 插入管理员账号配置
	hashedAdminPassword, _ := facades.Hash().Make("admin123")
	adminSettings := []map[string]interface{}{
		{
			"setting_key":   "panel_title",
			"setting_value": "CloudSentinel 云哨",
			"setting_type":  "string",
			"description":   "面板标题",
		},
		{
			"setting_key":   "admin_username",
			"setting_value": "admin",
			"setting_type":  "string",
			"description":   "管理员用户名",
		},
		{
			"setting_key":   "admin_password_hash",
			"setting_value": hashedAdminPassword,
			"setting_type":  "string",
			"description":   "管理员密码哈希",
		},
		{
			"setting_key":   "version",
			"setting_value": "1.0.0",
			"setting_type":  "string",
			"description":   "当前版本号",
		},
		// 权限相关设置
		{
			"setting_key":   "session_timeout",
			"setting_value": "3600",
			"setting_type":  "number",
			"description":   "会话超时时间（秒）",
		},
		{
			"setting_key":   "max_login_attempts",
			"setting_value": "5",
			"setting_type":  "number",
			"description":   "最大登录尝试次数",
		},
		{
			"setting_key":   "lockout_duration",
			"setting_value": "900",
			"setting_type":  "number",
			"description":   "账户锁定时间（秒）",
		},
		{
			"setting_key":   "jwt_secret",
			"setting_value": "cloudsentinel-secret-key-change-in-production",
			"setting_type":  "string",
			"description":   "JWT签名密钥",
		},
		{
			"setting_key":   "jwt_expiration",
			"setting_value": "86400",
			"setting_type":  "number",
			"description":   "JWT过期时间（秒）",
		},
		// 访客访问相关设置
		{
			"setting_key":   "allow_guest_login",
			"setting_value": "false",
			"setting_type":  "boolean",
			"description":   "是否允许游客登录",
		},
		{
			"setting_key":   "guest_password_enabled",
			"setting_value": "true",
			"setting_type":  "boolean",
			"description":   "是否启用游客密码访问",
		},
		{
			"setting_key":   "guest_password_hash",
			"setting_value": "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
			"setting_type":  "string",
			"description":   "游客访问密码哈希（默认：admin123）",
		},
		{
			"setting_key":   "hide_sensitive_info",
			"setting_value": "true",
			"setting_type":  "boolean",
			"description":   "是否隐藏敏感信息",
		},
	}

	// 先清空表，避免重复插入
	facades.Orm().Query().Table("system_settings").Delete()

	// 获取当前Unix时间戳
	now := time.Now().Unix()

	for _, setting := range adminSettings {
		// 添加时间戳字段
		setting["created_at"] = now
		setting["updated_at"] = now

		err := facades.Orm().Query().Table("system_settings").Create(setting)
		if err != nil {
			return err
		}
	}

	return nil
}

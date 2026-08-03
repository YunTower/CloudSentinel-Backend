package config

import (
	"github.com/goravel/framework/support/carbon"
	"goravel/app/facades"
)

// Boot Start all init methods of the current folder to bootstrap all config.
func Boot() {}

func init() {
	config := facades.Config()

	config.Add("app", map[string]any{
		// Application Name
		//
		// This value is the name of your application. This value is used when the
		// framework needs to place the application's name in a notification or
		// any other location as required by the application or its packages.
		"name":    config.Env("APP_NAME", "CloudSentinel"),
		"version": "0.0.1-beta.2",

		// Application Environment
		//
		// This value determines the "environment" your application is currently
		// running in. This may determine how you prefer to configure various
		// services the application utilizes. Set this in your ".env" file.
		"env": config.Env("APP_ENV", "production"),

		// Application Debug Mode
		"debug": config.Env("APP_DEBUG", false),

		// Disabled Runners
		//
		// Runner 签名支持 glob 匹配。默认不禁用任何 Runner，以保持 HTTP、
		// Schedule、Queue 与 CloudSentinel 后台服务的现有运行行为。
		"disabled_runners": []string{},

		// Maintenance Mode
		//
		// 单机部署默认使用文件驱动；多实例部署可切换为 cache 驱动。
		"maintenance": map[string]any{
			"driver": config.Env("APP_MAINTENANCE_DRIVER", "file"),
			"store":  config.Env("APP_MAINTENANCE_STORE", ""),
		},

		// Application Timezone
		//
		// Here you may specify the default timezone for your application.
		// Example: UTC, Asia/Shanghai
		// More: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
		"timezone": carbon.UTC,

		// Application Locale Configuration (not used: translation service provider is disabled)
		// "locale": "en",
		// "fallback_locale": "en",
		// "lang_path": "lang",

		// Encryption Key
		//
		// 32 character string, otherwise these encrypted strings
		// will not be safe. Please do this before deploying an application!
		"key": config.Env("APP_KEY", ""),
	})
}

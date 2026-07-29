package config

import (
	"github.com/goravel/framework/facades"
)

func init() {
	config := facades.Config()
	config.Add("cors", map[string]any{
		// Cross-Origin Resource Sharing (CORS) Configuration
		//
		// Here you may configure your settings for cross-origin resource sharing
		// or "CORS". This determines what cross-origin operations may execute
		// in web browsers. You are free to adjust these settings as needed.
		//
		// To learn more: https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
		"paths":                []string{},
		"allowed_methods":      []string{"*"},
		"allowed_origins":      []string{},
		"allowed_headers":      []string{"*"},
		"exposed_headers":      []string{},
		"max_age":              0,
		"supports_credentials": true,
		// 逗号分隔；须经 facades.Config 读取（.env 不会写入 os.Environ）。
		"admin_origins":  config.Env("CORS_ADMIN_ORIGINS", ""),
		"public_origins": config.Env("CORS_PUBLIC_ORIGINS", ""),
	})
}

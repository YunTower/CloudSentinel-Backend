package routes

import (
	"github.com/goravel/framework/facades"

	"goravel/app/http/controllers"
	"goravel/app/http/middleware"
)

func Api() {
	authController := controllers.NewAuthController()
	settingsController := controllers.NewSettingsController()

	// 公开路由
	facades.Route().Post("/auth/login", authController.Login)
	facades.Route().Get("/settings/public", settingsController.GetPublicSettings)

	// 需要认证的路由
	facades.Route().Middleware(middleware.SimpleAuth()).Get("/auth/refresh", authController.Refresh)
	facades.Route().Middleware(middleware.SimpleAuth()).Get("/auth/check", authController.Check)
}

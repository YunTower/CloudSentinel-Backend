package routes

import (
	"github.com/goravel/framework/facades"

	"goravel/app/http/controllers"
	"goravel/app/http/middleware"
)

func Api() {
	authController := controllers.NewAuthController()
	
	// 公开路由（无需认证）
	facades.Route().Post("/auth/login", authController.Login)
	
	// 需要认证的路由
	facades.Route().Middleware(middleware.SimpleAuth()).Get("/auth/refresh", authController.Refresh)
	facades.Route().Middleware(middleware.SimpleAuth()).Get("/auth/check", authController.Check)
}

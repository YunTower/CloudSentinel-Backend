package bootstrap

import (
	"github.com/goravel/framework/contracts/foundation/configuration"

	appmiddleware "goravel/app/http/middleware"
)

// Middleware 注册全局 HTTP 中间件；路由级中间件在 routes 中就近声明。
func Middleware(handler configuration.Middleware) {
	handler.Append(appmiddleware.CORS())
}

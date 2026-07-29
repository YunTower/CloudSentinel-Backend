package middleware

import (
	"net/http"
	"strings"

	contracts "github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

// CORS 使用两份白名单：公开站只可读取公开接口，管理站才可携带会话凭据访问管理接口。
// 空白名单是安全默认值：同源请求仍可用，任何跨域浏览器请求都会被拒绝。
func CORS() contracts.Middleware {
	adminOrigins := originSet(facades.Config().GetString("cors.admin_origins"))
	publicOrigins := originSet(facades.Config().GetString("cors.public_origins"))

	return func(ctx contracts.Context) {
		origin := strings.TrimSpace(ctx.Request().Header("Origin"))
		if origin == "" {
			ctx.Request().Next()
			return
		}

		isPublicRoute := isPublicAPIPath(ctx.Request().Path())
		_, isAdminOrigin := adminOrigins[origin]
		_, isPublicOrigin := publicOrigins[origin]
		allowed := isAdminOrigin || (isPublicRoute && isPublicOrigin)
		if !allowed {
			_ = ctx.Response().Status(http.StatusForbidden).Json(contracts.Json{
				"status":  false,
				"message": "不受信任的请求来源",
			}).Abort()
			return
		}

		response := ctx.Response().Header("Access-Control-Allow-Origin", origin).
			Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS").
			Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token").
			Header("Access-Control-Max-Age", "600").
			Header("Vary", "Origin")
		if isAdminOrigin {
			response.Header("Access-Control-Allow-Credentials", "true")
		}

		if ctx.Request().Method() == http.MethodOptions {
			_ = response.NoContent(http.StatusNoContent).Abort()
			return
		}

		ctx.Request().Next()
	}
}

func originSet(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, value := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(value)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func isPublicAPIPath(path string) bool {
	path = strings.TrimPrefix(path, "/")
	return path == "api/settings/public" || strings.HasPrefix(path, "api/public/")
}

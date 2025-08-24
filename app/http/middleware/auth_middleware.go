package middleware

import (
	"fmt"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

// parseToken 统一解析 Authorization token
func parseToken(ctx http.Context) (string, error) {
	authHeader := ctx.Request().Header("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("缺少认证令牌")
	}

	// 移除 "Bearer " 前缀（如果存在）
	token := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	return token, nil
}

// Auth 统一认证中间件
func Auth() http.Middleware {
	return func(ctx http.Context) {
		// 使用统一函数解析 token
		token, err := parseToken(ctx)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": err.Error(),
			})
			return
		}

		// 解析 Token
		payload, err := facades.Auth(ctx).Parse(token)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "Token无效或已过期",
			})
			return
		}

		// 设置用户信息到上下文，供后续处理使用
		ctx.WithValue("user_id", payload.Key)
		ctx.WithValue("guard", payload.Guard)

		// 设置用户类型
		userType := "guest"
		if payload.Key == "1" {
			userType = "admin"
		}
		ctx.WithValue("user_type", userType)
		ctx.WithValue("is_authenticated", true)

		// Token 有效，继续处理请求
		ctx.Request().Next()
	}
}

// SimpleAuth 简化的认证检查，适用于不需要复杂权限的场景
func SimpleAuth() http.Middleware {
	return func(ctx http.Context) {
		// 使用统一函数解析 token
		token, err := parseToken(ctx)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "用户未认证",
			})
			return
		}

		payload, err := facades.Auth(ctx).Parse(token)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "用户未认证",
			})
			return
		}

		// 设置用户信息到上下文，供后续处理使用
		ctx.WithValue("user_id", payload.Key)
		ctx.WithValue("guard", payload.Guard)

		// 设置用户类型
		userType := "guest"
		if payload.Key == "1" {
			userType = "admin"
		}
		ctx.WithValue("user_type", userType)
		ctx.WithValue("is_authenticated", true)

		ctx.Request().Next()
	}
}

// AdminAuth 管理员权限检查
func AdminAuth() http.Middleware {
	return func(ctx http.Context) {
		// 使用统一函数解析 token
		token, err := parseToken(ctx)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "用户未认证",
			})
			return
		}

		payload, err := facades.Auth(ctx).Parse(token)
		if err != nil {
			ctx.Response().Status(401).Json(http.Json{
				"status":  false,
				"message": "用户未认证",
			})
			return
		}

		// 检查是否是管理员
		if payload.Key != "1" {
			ctx.Response().Status(403).Json(http.Json{
				"status":  false,
				"message": "权限不足",
			})
			return
		}

		ctx.Request().Next()
	}
}

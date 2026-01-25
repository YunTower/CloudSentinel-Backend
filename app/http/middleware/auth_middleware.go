package middleware

import (
	"fmt"
	"reflect"

	"goravel/app/utils"

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

// setUserContext 设置用户信息到上下文
func setUserContext(ctx http.Context, payload interface{}) {
	key := getPayloadField(payload, "Key")
	guard := getPayloadField(payload, "Guard")

	ctx.WithValue("user_id", key)
	ctx.WithValue("guard", guard)

	userType := "guest"
	if guard == "admin" {
		userType = "admin"
	}
	ctx.WithValue("user_type", userType)
	ctx.WithValue("is_authenticated", true)
}

// getPayloadField 通过反射获取 payload 字段值
func getPayloadField(payload interface{}, fieldName string) string {
	v := reflect.ValueOf(payload)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName(fieldName)
	if !f.IsValid() || !f.CanInterface() {
		return ""
	}
	if f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}

// handleAuthError 处理认证错误
func handleAuthError(ctx http.Context, err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	if errMsg == "token has expired" || errMsg == "token expired" {
		utils.ErrorResponse(ctx, 401, "Token已过期", "TOKEN_EXPIRED")
		return true
	}

	utils.ErrorResponse(ctx, 401, "Token无效", "TOKEN_INVALID")
	return true
}

// authenticate 公共认证逻辑
func authenticate(ctx http.Context, requireAdmin bool) bool {
	token, err := parseToken(ctx)
	if err != nil {
		utils.ErrorResponse(ctx, 401, err.Error(), "TOKEN_MISSING")
		return false
	}

	payload, err := facades.Auth(ctx).Parse(token)
	if err != nil {
		return !handleAuthError(ctx, err)
	}

	guard := getPayloadField(payload, "Guard")

	if requireAdmin && guard != "admin" {
		utils.ErrorResponse(ctx, 403, "权限不足", "INSUFFICIENT_PERMISSIONS")
		return false
	}

	// 设置用户信息
	setUserContext(ctx, payload)
	return true
}

// Auth 统一认证中间件
func Auth() http.Middleware {
	return func(ctx http.Context) {
		if authenticate(ctx, false) {
			ctx.Request().Next()
		}
	}
}

// SimpleAuth 简单认证检查
func SimpleAuth() http.Middleware {
	return func(ctx http.Context) {
		if authenticate(ctx, false) {
			ctx.Request().Next()
		}
	}
}

// AdminAuth 管理员权限检查
func AdminAuth() http.Middleware {
	return func(ctx http.Context) {
		if authenticate(ctx, true) {
			ctx.Request().Next()
		}
	}
}

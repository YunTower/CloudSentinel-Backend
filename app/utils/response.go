package utils

import (
	"github.com/goravel/framework/contracts/http"
)

// ErrorResponse 返回错误响应
func ErrorResponse(ctx http.Context, status int, message string, code ...string) http.Response {
	response := http.Json{
		"status":  false,
		"message": message,
	}
	if len(code) > 0 && code[0] != "" {
		response["code"] = code[0]
	}
	return ctx.Response().Status(status).Json(response)
}

// ErrorResponseWithError 返回带错误详情的错误响应
func ErrorResponseWithError(ctx http.Context, status int, message string, err error, code ...string) http.Response {
	response := http.Json{
		"status":  false,
		"message": message,
		"error":   err.Error(),
	}
	if len(code) > 0 && code[0] != "" {
		response["code"] = code[0]
	}
	return ctx.Response().Status(status).Json(response)
}

// SuccessResponse 返回成功响应
func SuccessResponse(ctx http.Context, message string, data ...interface{}) http.Response {
	response := http.Json{
		"status":  true,
		"message": message,
	}
	if len(data) > 0 && data[0] != nil {
		response["data"] = data[0]
	}
	return ctx.Response().Success().Json(response)
}

// SuccessResponseWithStatus 返回带状态码的成功响应
func SuccessResponseWithStatus(ctx http.Context, status int, message string, data ...interface{}) http.Response {
	response := http.Json{
		"status":  true,
		"message": message,
	}
	if len(data) > 0 && data[0] != nil {
		response["data"] = data[0]
	}
	return ctx.Response().Status(status).Json(response)
}


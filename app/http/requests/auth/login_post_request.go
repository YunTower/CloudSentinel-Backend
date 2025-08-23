package auth

import (
	"github.com/goravel/framework/contracts/http"
)

type LoginPostRequest struct {
	Type     string `form:"type" json:"type"`
	Password string `form:"password" json:"password"`
	Username string `form:"username" json:"username"`
	Remember bool   `form:"remember" json:"remember"`
}

// Authorize 授权验证
func (r *LoginPostRequest) Authorize(ctx http.Context) error {
	// 这里可以添加授权逻辑，比如检查用户是否有登录权限
	// 目前返回 nil，表示允许所有用户登录
	return nil
}

// Rules 验证规则
func (r *LoginPostRequest) Rules(ctx http.Context) map[string]string {
	return map[string]string{
		"type":     "required|in:admin,guest",
		"password": "required|min_len:6",
		"username": "required_if:type,admin|min_len:3|max_len:50",
		"remember": "boolean",
	}
}

// Messages 自定义错误消息
func (r *LoginPostRequest) Messages() map[string]string {
	return map[string]string{
		"type.required":     "用户类型不能为空",
		"type.in":           "用户类型必须是 admin 或 guest",
		"password.required": "密码不能为空",
		"password.min_len":  "密码长度不能少于6位",
		"username.required_if": "管理员用户必须提供用户名",
		"username.min_len":  "用户名长度不能少于3位",
		"username.max_len":  "用户名长度不能超过50位",
		"remember.boolean":  "记住我字段必须是布尔值",
	}
}

// Attributes 自定义验证属性名称
func (r *LoginPostRequest) Attributes() map[string]string {
	return map[string]string{
		"type":     "用户类型",
		"password": "密码",
		"username": "用户名",
		"remember": "记住我",
	}
}

// Filters 输入数据过滤
func (r *LoginPostRequest) Filters(ctx http.Context) map[string]string {
	return map[string]string{
		"username": "trim",
		"type":     "trim",
	}
}

// PrepareForValidation 验证前准备数据
func (r *LoginPostRequest) PrepareForValidation(ctx http.Context) {
	// 在验证前对数据进行预处理
	// 比如转换大小写、去除空格等
}

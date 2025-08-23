package controllers

import (
	"fmt"
	"goravel/app/http/requests/auth"
	"goravel/app/models"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/facades"
)

type AuthController struct {
	Type     string `form:"type" json:"type"`
	Password string `form:"password" json:"password"`
	Username string `form:"username" json:"username"`
	Remember bool   `form:"remember" json:"remember"`
}

func NewAuthController() *AuthController {
	return &AuthController{
		// Inject services
	}
}

func (r *AuthController) Login(ctx http.Context) http.Response {
	var loginPost auth.LoginPostRequest
	
	// 使用 ValidateRequest 方法验证表单请求
	errors, err := ctx.Request().ValidateRequest(&loginPost)
	if err != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "验证器创建失败",
			"error":   err.Error(),
		})
	}

	if errors != nil {
		return ctx.Response().Status(422).Json(http.Json{
			"status":  false,
			"message": "验证失败",
			"errors":  errors,
		})
	}

	// 查询用户名
	var userName string
	userNameErr := facades.DB().Table("system_settings").Where("setting_key", "admin_username").Value("setting_value", &userName)
	if userNameErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询用户名失败",
			"error":   userNameErr.Error(),
		})
	}

	// 检查用户名是否为空
	if userName == "" {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "用户名配置不存在",
		})
	}
	fmt.Println("查询到的用户名:", userName)

	// 查询密码哈希
	var userPasswordHash string
	userPasswordErr := facades.DB().Table("system_settings").Where("setting_key", "admin_password_hash").Value("setting_value", &userPasswordHash)
	if userPasswordErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "查询密码配置失败",
			"error":   userPasswordErr.Error(),
		})
	}

	// 检查密码哈希是否为空
	if userPasswordHash == "" {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "密码配置不存在",
		})
	}
	fmt.Println("查询到的密码哈希:", userPasswordHash)

	// 验证用户名
	if loginPost.Username != userName {
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "用户名错误",
		})
	}

	// 验证密码哈希
	if facades.Hash().Check(loginPost.Password, userPasswordHash) != true {
		return ctx.Response().Status(401).Json(http.Json{
			"status":  false,
			"message": "密码错误",
		})
	}

	// 创建用户模型用于认证
	ua := ctx.Request().Header("User-Agent")
	ip := ctx.Request().Ip()

	// 创建 User 模型实例
	user := &models.User{
		Username: loginPost.Username,
		Type:     loginPost.Type,
		IP:       ip,
		UA:       ua,
	}
	// 设置 ID 字段，这是认证必需的
	user.ID = uint(1) // 使用固定 ID，因为我们没有真正的用户表

	// 使用 facades.Auth() 生成 JWT token
	token, tokenErr := facades.Auth(ctx).Login(user)
	if tokenErr != nil {
		return ctx.Response().Status(500).Json(http.Json{
			"status":  false,
			"message": "Token生成失败",
			"error":   tokenErr.Error(),
		})
	}

	return ctx.Response().Success().Json(http.Json{
		"status":  true,
		"message": "登录成功",
		"data": map[string]any{
			"token":    token,
			"username": loginPost.Username,
			"type":     loginPost.Type,
		},
	})
}

package controllers

import (
	"fmt"
	"goravel/app/http/requests/auth"
	"goravel/app/models"
	"time"

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

	if loginPost.Type == "admin" {

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
	} else {
		// 检查是否允许游客登录
		var allowGuestLogin string
		guestLoginErr := facades.DB().Table("system_settings").Where("setting_key", "allow_guest_login").Value("setting_value", &allowGuestLogin)
		if guestLoginErr != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "查询游客登录配置失败",
				"error":   guestLoginErr.Error(),
			})
		}

		if allowGuestLogin != "true" {
			return ctx.Response().Status(403).Json(http.Json{
				"status":  false,
				"message": "游客登录功能已禁用",
			})
		}

		// 检查是否开启游客密码访问
		var guestPasswordEnabled string
		guestPasswordErr := facades.DB().Table("system_settings").Where("setting_key", "guest_password_enabled").Value("setting_value", &guestPasswordEnabled)
		if guestPasswordErr != nil {
			return ctx.Response().Status(500).Json(http.Json{
				"status":  false,
				"message": "查询游客密码配置失败",
				"error":   guestPasswordErr.Error(),
			})
		}

		// 如果开启密码访问，验证密码
		if guestPasswordEnabled == "true" {
			var guestPasswordHash string
			guestPasswordHashErr := facades.DB().Table("system_settings").Where("setting_key", "guest_password_hash").Value("setting_value", &guestPasswordHash)
			if guestPasswordHashErr != nil {
				return ctx.Response().Status(500).Json(http.Json{
					"status":  false,
					"message": "查询游客密码配置失败",
					"error":   guestPasswordHashErr.Error(),
				})
			}

			if guestPasswordHash == "" {
				return ctx.Response().Status(500).Json(http.Json{
					"status":  false,
					"message": "游客密码配置不存在",
				})
			}

			// 验证游客密码
			if facades.Hash().Check(loginPost.Password, guestPasswordHash) != true {
				return ctx.Response().Status(401).Json(http.Json{
					"status":  false,
					"message": "游客密码错误",
				})
			}
		}

		loginPost.Username = "guest"
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
	// 根据用户类型设置不同的 ID
	if loginPost.Type == "admin" {
		user.ID = 1 // 管理员使用固定 ID 1
	} else {
		// 游客使用动态 ID，基于时间戳和随机数生成唯一标识
		user.ID = uint(time.Now().UnixNano()%1000000 + 100000) // 生成 100000-1099999 范围内的唯一 ID
	}

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

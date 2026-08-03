package auth

import (
	"context"
	"testing"

	"github.com/goravel/framework/contracts/http"
	frameworkvalidation "github.com/goravel/framework/validation"
)

var (
	_ http.FormRequest                         = (*LoginPostRequest)(nil)
	_ http.FormRequestWithFilters              = (*LoginPostRequest)(nil)
	_ http.FormRequestWithMessages             = (*LoginPostRequest)(nil)
	_ http.FormRequestWithAttributes           = (*LoginPostRequest)(nil)
	_ http.FormRequestWithPrepareForValidation = (*LoginPostRequest)(nil)
)

func TestLoginPostRequestRulesWithNativeValidation(t *testing.T) {
	request := &LoginPostRequest{}
	validation := frameworkvalidation.NewValidation()

	validator, err := validation.Make(
		context.Background(),
		map[string]any{
			"type":     " admin ",
			"username": " operator ",
			"password": "secret12",
			"remember": true,
		},
		request.Rules(nil),
		frameworkvalidation.Filters(request.Filters(nil)),
		frameworkvalidation.Messages(request.Messages(nil)),
		frameworkvalidation.Attributes(request.Attributes(nil)),
	)
	if err != nil {
		t.Fatalf("创建登录验证器失败: %v", err)
	}
	if validator.Fails() {
		t.Fatalf("合法登录数据验证失败: %v", validator.Errors().All())
	}

	validator, err = validation.Make(
		context.Background(),
		map[string]any{
			"type":     "admin",
			"username": "ab",
			"password": "short",
		},
		request.Rules(nil),
		frameworkvalidation.Messages(request.Messages(nil)),
	)
	if err != nil {
		t.Fatalf("创建非法登录数据验证器失败: %v", err)
	}
	if !validator.Fails() {
		t.Fatal("非法登录数据应验证失败")
	}
	if got := validator.Errors().Get("username")["min"]; got != "用户名长度不能少于3位" {
		t.Fatalf("用户名 min 消息不匹配: %q", got)
	}
	if got := validator.Errors().Get("password")["min"]; got != "密码长度不能少于6位" {
		t.Fatalf("密码 min 消息不匹配: %q", got)
	}
}

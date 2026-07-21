package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/goravel/framework/contracts/http"
)

const CSRFTokenCookieName = "__Host-csrf"

func NewCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// VerifyCSRF protects state-changing browser requests authenticated by cookies.
func VerifyCSRF() http.Middleware {
	return func(ctx http.Context) {
		method := ctx.Request().Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			ctx.Request().Next()
			return
		}
		cookie := ctx.Request().Cookie(CSRFTokenCookieName)
		header := ctx.Request().Header("X-CSRF-Token")
		if cookie == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
			_ = ctx.Response().Status(403).Json(http.Json{"status": false, "message": "CSRF 校验失败"})
			return
		}
		ctx.Request().Next()
	}
}

func RequireCSRFToken() (string, error) {
	token, err := NewCSRFToken()
	if err != nil {
		return "", fmt.Errorf("生成 CSRF 令牌失败: %w", err)
	}
	return token, nil
}

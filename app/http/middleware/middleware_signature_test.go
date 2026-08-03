package middleware

import (
	"testing"
	"time"

	"github.com/goravel/framework/contracts/http"
)

func TestMiddlewareSignaturesAreUnique(t *testing.T) {
	middlewares := []http.Middleware{
		Auth(),
		AdminAuth(),
		Public(),
		VerifyCSRF(),
		RateLimit("test", 1, time.Minute),
	}

	seen := make(map[string]struct{}, len(middlewares))
	for _, item := range middlewares {
		signature := item.Signature()
		if signature == "" {
			t.Fatal("middleware signature must not be empty")
		}
		if _, exists := seen[signature]; exists {
			t.Fatalf("middleware signature %q is duplicated", signature)
		}
		seen[signature] = struct{}{}
	}
}

func TestRateLimitSignatureUsesScope(t *testing.T) {
	if got := RateLimit("login", 10, time.Minute).Signature(); got != "cloudsentinel:rate_limit:login" {
		t.Fatalf("unexpected rate limit signature: %q", got)
	}
}

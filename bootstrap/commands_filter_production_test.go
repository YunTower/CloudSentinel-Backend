//go:build production

package bootstrap

import (
	"slices"
	"testing"
)

func TestCommandsFilterUsesProductionAllowlist(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	got := CommandsFilter()

	for _, signature := range []string{"list", "up", "down", "migrate"} {
		if !slices.Contains(got, signature) {
			t.Fatalf("生产构建命令列表缺少 %q", signature)
		}
	}

	for _, signature := range []string{"make:*", "package:*", "vendor:publish", "migrate:*", "start", "update"} {
		if slices.Contains(got, signature) {
			t.Fatalf("生产构建不应放行 %q", signature)
		}
	}
}

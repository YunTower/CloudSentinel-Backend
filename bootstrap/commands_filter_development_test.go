//go:build !production

package bootstrap

import "testing"

func TestCommandsFilterKeepsAllCommandsInDevelopmentBuild(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	if got := CommandsFilter(); got != nil {
		t.Fatalf("普通构建应保留全部命令，got %v", got)
	}
}

//go:build !production

package bootstrap

// CommandsFilter 在普通构建中保留全部 Artisan 命令，供开发和调试使用。
func CommandsFilter() []string {
	return nil
}

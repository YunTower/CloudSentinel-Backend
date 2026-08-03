//go:build production

package bootstrap

// CommandsFilter 为生产构建提供 Artisan 命令正向列表。
// list 是程序无参数启动时的默认命令，migrate 是应用自更新流程的必需命令。
func CommandsFilter() []string {
	return []string{
		"about",
		"list",
		"up",
		"down",
		"key:generate",
		"migrate",
		"queue:*",
		"schedule:*",
	}
}

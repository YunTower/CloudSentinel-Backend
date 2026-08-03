package bootstrap

import (
	"github.com/goravel/framework/contracts/console"

	"goravel/app/console/commands"
)

// Commands 注册 CloudSentinel 自定义命令。构建类型差异统一由 CommandsFilter 处理。
func Commands() []console.Command {
	return []console.Command{
		commands.NewStartCommand(),
		commands.NewStopCommand(),
		commands.NewRestartCommand(),
		commands.NewResetPortCommand(),
		commands.NewGenerateAdminCommand(),
		commands.NewPanelInfoCommand(),
		commands.NewUninstallCommand(),
		commands.NewUpdateCommand(),
		commands.NewGenerateCertCommand(),
		commands.NewRenewCertCommand(),
		commands.NewInfoCertCommand(),
	}
}

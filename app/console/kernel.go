package console

import (
	"goravel/app/console/commands"
	"goravel/app/jobs"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/schedule"
	"github.com/goravel/framework/facades"
)

type Kernel struct {
}

func (kernel Kernel) Schedule() []schedule.Event {
	return []schedule.Event{
		// 每天凌晨 1 点检查服务器到期告警
		facades.Schedule().Call(func() {
			job := &jobs.CheckServerExpirationJob{}
			if err := job.Handle(); err != nil {
				facades.Log().Errorf("执行服务器到期检查任务失败: %v", err)
			}
		}).DailyAt("01:00").Name("check_server_expiration"),
	}
}

func (kernel Kernel) Commands() []console.Command {
	return []console.Command{
		commands.NewStartCommand(),
		commands.NewStopCommand(),
		commands.NewRestartCommand(),
		commands.NewResetPortCommand(),
		commands.NewGenerateAdminCommand(),
		commands.NewPanelInfoCommand(),
		commands.NewUninstallCommand(),
		commands.NewUpdateCommand(),
	}
}

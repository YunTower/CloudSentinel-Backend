package bootstrap

import (
	"github.com/goravel/framework/contracts/schedule"

	"goravel/app/facades"
	"goravel/app/jobs"
)

func Schedule() []schedule.Event {
	return []schedule.Event{
		facades.Schedule().Call(func() {
			job := &jobs.CheckServerExpirationJob{}
			if err := job.Handle(); err != nil {
				facades.Log().Errorf("执行服务器到期检查任务失败: %v", err)
			}
		}).DailyAt("01:00").Name("check_server_expiration"),
	}
}

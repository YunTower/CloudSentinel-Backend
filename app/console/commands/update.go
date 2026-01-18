package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"goravel/app/http/controllers"
	"goravel/app/services"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/facades"
)

type UpdateCommand struct {
	updateService *services.UpdateService
}

func NewUpdateCommand() *UpdateCommand {
	return &UpdateCommand{
		updateService: services.NewUpdateService(),
	}
}

// Signature The name and signature of the console command.
func (c *UpdateCommand) Signature() string {
	return "update"
}

// Description The console command description.
func (c *UpdateCommand) Description() string {
	return "更新 CloudSentinel Dashboard 到最新版本"
}

// Extend The console command extend.
func (c *UpdateCommand) Extend() command.Extend {
	return command.Extend{
		Flags: []command.Flag{
			&command.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "强制更新（即使已是最新版本）",
			},
			&command.BoolFlag{
				Name:  "skip-migration",
				Usage: "跳过数据库迁移",
			},
			&command.BoolFlag{
				Name:  "yes",
				Aliases: []string{"y"},
				Usage: "跳过确认提示",
			},
		},
	}
}

// Handle Execute the console command.
func (c *UpdateCommand) Handle(ctx console.Context) error {
	forceFlag := ctx.Option("force") == "true" || ctx.Option("force") == "1"
	skipMigrationFlag := ctx.Option("skip-migration") == "true" || ctx.Option("skip-migration") == "1"
	yesFlag := ctx.Option("yes") == "true" || ctx.Option("yes") == "1" || ctx.Option("y") == "true" || ctx.Option("y") == "1"

	// 检查是否有更新在进行中
	if !forceFlag {
		if facades.Cache().Has("update_status") {
			cachedValue := facades.Cache().Get("update_status", nil)
			if cachedValue != nil {
				var status controllers.UpdateStatus
				if cachedStatus, ok := cachedValue.(controllers.UpdateStatus); ok {
					status = cachedStatus
				} else {
					if err := facades.Cache().Get("update_status", &status); err == nil {
						activeSteps := map[string]bool{
							"connecting":  true,
							"downloading": true,
							"verifying":   true,
							"unpacking":   true,
							"restarting":  true,
						}

						if activeSteps[status.Step] {
							PrintError("更新已在进行中，请稍后再试")
							return fmt.Errorf("更新已在进行中")
						}
					}
				}
			}
		}
	}

	// 用户确认
	if !yesFlag {
		PrintWarning("此操作将更新 CloudSentinel Dashboard 到最新版本")
		PrintWarning("更新过程中会备份数据库文件和二进制文件")
		PrintWarning("更新完成后将自动重启服务")
		fmt.Println()
		fmt.Print("确定要继续吗？(y/N): ")

		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			PrintError("读取输入失败")
			return err
		}

		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			PrintInfo("已取消")
			return nil
		}
		fmt.Println()
	}

	// 使用更新服务执行更新
	statusCallback := func(step string, progress int, message string) {
		switch step {
		case "connecting":
			PrintInfo(message)
		case "downloading":
			if progress%10 == 0 || progress == 100 {
				PrintInfo(fmt.Sprintf("%s (%d%%)", message, progress))
			}
		case "verifying":
			PrintInfo(message)
		case "unpacking":
			PrintInfo(message)
		case "migrating":
			PrintInfo(message)
		case "restarting":
			PrintInfo(message)
			PrintWarning("服务重启后，当前命令将退出")
		case "completed":
			PrintSuccess(message)
		case "error":
			PrintError(message)
		default:
			PrintInfo(message)
		}
	}

	options := services.UpdateOptions{
		Force:          forceFlag,
		SkipMigration: skipMigrationFlag,
		StatusCallback: statusCallback,
		ReleaseURL:     "https://api.github.com/repos/YunTower/CloudSentinel/releases/latest",
	}

	if err := c.updateService.ExecuteUpdate(options); err != nil {
		return err
	}

	return nil
}


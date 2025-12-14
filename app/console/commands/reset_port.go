package commands

import (
	"fmt"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
)

type ResetPortCommand struct{}

func NewResetPortCommand() *ResetPortCommand {
	return &ResetPortCommand{}
}

// Signature The name and signature of the console command.
func (c *ResetPortCommand) Signature() string {
	return "reset-port"
}

// Description The console command description.
func (c *ResetPortCommand) Description() string {
	return "重置面板端口为随机端口（8000-65535）"
}

// Extend The console command extend.
func (c *ResetPortCommand) Extend() command.Extend {
	return command.Extend{}
}

// Handle Execute the console command.
func (c *ResetPortCommand) Handle(ctx console.Context) error {
	PrintInfo("正在生成随机端口...")

	// 生成随机端口
	port, err := GenerateRandomPort()
	if err != nil {
		PrintError(fmt.Sprintf("生成随机端口失败: %v", err))
		return err
	}

	PrintInfo(fmt.Sprintf("新端口: %d", port))

	// 更新 .env 文件
	if err := UpdatePortInEnv(port); err != nil {
		PrintError(fmt.Sprintf("更新端口配置失败: %v", err))
		return err
	}

	PrintSuccess(fmt.Sprintf("端口已重置为: %d", port))
	PrintInfo("请使用 'restart' 命令重启服务以使新端口生效")

	return nil
}


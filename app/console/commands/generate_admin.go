package commands

import (
	"fmt"

	"goravel/app/repositories"

	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/console/command"
	"github.com/goravel/framework/facades"
)

type GenerateAdminCommand struct{}

func NewGenerateAdminCommand() *GenerateAdminCommand {
	return &GenerateAdminCommand{}
}

// Signature The name and signature of the console command.
func (c *GenerateAdminCommand) Signature() string {
	return "generate:admin"
}

// Description The console command description.
func (c *GenerateAdminCommand) Description() string {
	return "生成随机管理员用户名和密码（10位用户名，20位密码）"
}

// Extend The console command extend.
func (c *GenerateAdminCommand) Extend() command.Extend {
	return command.Extend{}
}

// Handle Execute the console command.
func (c *GenerateAdminCommand) Handle(ctx console.Context) error {
	PrintInfo("正在生成随机管理员账号...")

	// 生成10位随机用户名（字母和数字）
	username, err := GenerateRandomString(10, "alphanumeric")
	if err != nil {
		PrintError(fmt.Sprintf("生成用户名失败: %v", err))
		return err
	}

	// 生成20位随机密码（字母、数字和特殊字符）
	password, err := GenerateRandomString(20, "alphanumeric_special")
	if err != nil {
		PrintError(fmt.Sprintf("生成密码失败: %v", err))
		return err
	}

	// 生成密码哈希
	passwordHash, err := facades.Hash().Make(password)
	if err != nil {
		PrintError(fmt.Sprintf("生成密码哈希失败: %v", err))
		return err
	}

	// 更新数据库
	settingRepo := repositories.GetSystemSettingRepository()

	// 更新用户名
	if err := settingRepo.SetValue("admin_username", username); err != nil {
		PrintError(fmt.Sprintf("更新用户名失败: %v", err))
		return err
	}

	// 更新密码哈希
	if err := settingRepo.SetValue("admin_password_hash", passwordHash); err != nil {
		PrintError(fmt.Sprintf("更新密码哈希失败: %v", err))
		return err
	}

	// 输出结果
	PrintSuccess("管理员账号已生成")
	fmt.Println()
	fmt.Printf("  %s用户名: %s%s\n", ColorCyan, username, ColorReset)
	fmt.Printf("  %s密码: %s%s\n", ColorCyan, password, ColorReset)
	fmt.Printf("ADMIN_CREDENTIALS|%s|%s\n", username, password)
	fmt.Println()
	PrintWarning("请妥善保管用户名和密码，建议立即登录并修改密码")

	return nil
}

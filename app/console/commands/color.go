package commands

import (
	"fmt"
	"os"
)

// ANSI 颜色码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
	BOLD        = "\033[1m"
)

// supportsColor 检查是否支持颜色输出
func supportsColor() bool {
	return os.Getenv("TERM") != "" && os.Getenv("NO_COLOR") == ""
}

// printColor 打印彩色文本
func printColor(color, text string) {
	if supportsColor() {
		fmt.Printf("%s%s%s\n", color, text, ColorReset)
	} else {
		fmt.Println(text)
	}
}

// PrintSuccess 打印成功信息（绿色）
func PrintSuccess(text string) {
	printColor(ColorGreen, "✓ "+text)
}

// PrintError 打印错误信息（红色）
func PrintError(text string) {
	printColor(ColorRed, "✗ "+text)
}

// PrintWarning 打印警告信息（黄色）
func PrintWarning(text string) {
	printColor(ColorYellow, "⚠ "+text)
}

// PrintInfo 打印信息（蓝色）
func PrintInfo(text string) {
	printColor(ColorBlue, "ℹ "+text)
}

// PrintStatus 打印状态信息（带颜色）
func PrintStatus(status, text string) {
	var color string
	switch status {
	case "running", "active", "success":
		color = ColorGreen
	case "stopped", "inactive":
		color = ColorYellow
	case "failed", "error":
		color = ColorRed
	case "starting", "activating":
		color = ColorCyan
	case "stopping", "deactivating":
		color = ColorYellow
	default:
		color = ColorWhite
	}
	printColor(color, text)
}

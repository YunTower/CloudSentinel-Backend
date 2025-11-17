package services

import (
	"goravel/app/utils"
)

// GetLogWriter 获取全局日志写入器
func GetLogWriter() *utils.LogWriter {
	return utils.GetLogWriter()
}

// LogToChannel 记录日志到指定通道
func LogToChannel(channel, level, message string, args ...interface{}) {
	utils.LogToChannel(channel, level, message, args...)
}

package services

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/goravel/framework/facades"
)

// CleanupStaleLogLocks 清理过期的日志锁文件
// 锁文件是日志轮转时创建的，如果进程异常退出可能残留
func CleanupStaleLogLocks() error {
	logDir := "storage/logs"

	// 确保日志目录存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil // 目录不存在，无需清理
	}

	// 读取日志目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		facades.Log().Warningf("读取日志目录失败: %v", err)
		return err
	}

	now := time.Now()
	cleanedCount := 0

	for _, entry := range entries {
		// 只处理 .log_lock 文件
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log_lock") {
			continue
		}

		lockFilePath := filepath.Join(logDir, entry.Name())

		// 获取文件信息
		info, err := entry.Info()
		if err != nil {
			facades.Log().Warningf("获取锁文件信息失败: %s, %v", lockFilePath, err)
			continue
		}

		// 检查文件修改时间，如果超过5分钟未修改，认为是过期锁文件
		// 正常的锁文件应该在日志轮转时被及时删除（通常几秒内）
		// 如果超过5分钟还存在，说明是残留的锁文件
		if now.Sub(info.ModTime()) > 5*time.Minute {
			if err := os.Remove(lockFilePath); err != nil {
				facades.Log().Warningf("删除过期锁文件失败: %s, %v", lockFilePath, err)
			} else {
				cleanedCount++
				facades.Log().Infof("已清理过期锁文件: %s", entry.Name())
			}
		} else {
			// 即使未超过5分钟，也尝试删除（可能是残留的）
			// 因为正常的轮转应该在几秒内完成
			// 如果删除失败（可能正在使用），静默忽略
			if err := os.Remove(lockFilePath); err == nil {
				cleanedCount++
				facades.Log().Infof("已清理锁文件: %s", entry.Name())
			}
		}
	}

	if cleanedCount > 0 {
		facades.Log().Infof("清理完成，共清理 %d 个过期锁文件", cleanedCount)
	}

	return nil
}

// StartPeriodicLogLockCleanup 启动定期清理日志锁文件的任务
// 每5分钟清理一次，避免日志轮转失败
func StartPeriodicLogLockCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		// 立即执行一次
		CleanupStaleLogLocks()

		for range ticker.C {
			CleanupStaleLogLocks()
		}
	}()
}

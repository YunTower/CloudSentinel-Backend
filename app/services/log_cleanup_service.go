package services

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/goravel/framework/facades"
)

var (
	cleanupMutex sync.Mutex
	lastCleanup  time.Time
)

// CleanupStaleLogLocks 清理过期的日志锁文件
func CleanupStaleLogLocks() error {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	// 防止过于频繁的清理（至少间隔1秒）
	now := time.Now()
	if !lastCleanup.IsZero() && now.Sub(lastCleanup) < 1*time.Second {
		return nil
	}
	lastCleanup = now

	logDir := "storage/logs"

	// 确保日志目录存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil // 目录不存在，无需清理
	}

	// 读取日志目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

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
			continue
		}

		// 检查文件修改时间
		fileAge := now.Sub(info.ModTime())

		// 如果锁文件超过3秒未修改，认为是残留的锁文件
		// 正常的日志轮转应该在几秒内完成
		// 有了日志队列后，写入是串行的，轮转应该更快完成
		if fileAge > 3*time.Second {
			// 尝试删除锁文件
			if err := os.Remove(lockFilePath); err != nil {
				// 如果删除失败，可能是文件正在被使用，跳过
				continue
			}
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		// 直接使用facades.Log，避免循环依赖
		// 清理服务本身不会产生大量日志，直接写入即可
		facades.Log().Channel("websocket").Debugf("清理了 %d 个过期的日志锁文件", cleanedCount)
	}

	return nil
}

// StartPeriodicLogLockCleanup 启动定期清理日志锁文件的任务
// 每5秒清理一次
func StartPeriodicLogLockCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// 立即执行一次
		CleanupStaleLogLocks()

		for range ticker.C {
			CleanupStaleLogLocks()
		}
	}()
}

// ForceCleanupLogLocks 强制清理所有日志锁文件（用于紧急情况）
func ForceCleanupLogLocks() error {
	cleanupMutex.Lock()
	defer cleanupMutex.Unlock()

	logDir := "storage/logs"

	// 确保日志目录存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil
	}

	// 读取日志目录
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	cleanedCount := 0

	for _, entry := range entries {
		// 只处理 .log_lock 文件
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log_lock") {
			continue
		}

		lockFilePath := filepath.Join(logDir, entry.Name())

		// 强制删除所有锁文件
		if err := os.Remove(lockFilePath); err == nil {
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		// 直接使用facades.Log，避免循环依赖
		facades.Log().Channel("websocket").Warningf("强制清理了 %d 个日志锁文件", cleanedCount)
	}

	return nil
}

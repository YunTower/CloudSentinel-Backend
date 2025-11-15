package utils

import (
	"sync"
	"time"

	"github.com/goravel/framework/facades"
)

// LogWriter 日志写入器
type LogWriter struct {
	buffer    []LogEntry
	bufferMu  sync.Mutex
	batchSize int
	interval  time.Duration
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// LogEntry 日志条目
type LogEntry struct {
	Channel string
	Level   string
	Message string
	Args    []interface{}
}

var (
	globalLogWriter *LogWriter
	logWriterOnce   sync.Once
)

// GetLogWriter 获取全局日志写入器（单例）
func GetLogWriter() *LogWriter {
	logWriterOnce.Do(func() {
		globalLogWriter = &LogWriter{
			buffer:    make([]LogEntry, 0, 100),
			batchSize: 50,                    // 批量写入大小
			interval:  200 * time.Millisecond, // 批量写入间隔
			stopChan:  make(chan struct{}),
		}
		globalLogWriter.Start()
		facades.Log().Infof("启动日志写入队列，批量大小: %d, 写入间隔: %v", globalLogWriter.batchSize, globalLogWriter.interval)
	})
	return globalLogWriter
}

// Start 启动日志写入器
func (w *LogWriter) Start() {
	w.wg.Add(1)
	go w.flushLoop()
}

// Stop 停止日志写入器
func (w *LogWriter) Stop() {
	close(w.stopChan)
	w.wg.Wait()
	// 刷新剩余日志
	w.flush()
}

// Enqueue 将日志加入队列
func (w *LogWriter) Enqueue(channel, level, message string, args ...interface{}) {
	w.bufferMu.Lock()
	defer w.bufferMu.Unlock()

	w.buffer = append(w.buffer, LogEntry{
		Channel: channel,
		Level:   level,
		Message: message,
		Args:    args,
	})

	// 如果缓冲区满了，立即刷新
	if len(w.buffer) >= w.batchSize {
		go w.flush()
	}
}

// flushLoop 定期刷新日志
func (w *LogWriter) flushLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.flush()
		case <-w.stopChan:
			return
		}
	}
}

// flush 刷新日志到文件
func (w *LogWriter) flush() {
	w.bufferMu.Lock()
	if len(w.buffer) == 0 {
		w.bufferMu.Unlock()
		return
	}

	// 复制缓冲区并清空
	entries := make([]LogEntry, len(w.buffer))
	copy(entries, w.buffer)
	w.buffer = w.buffer[:0]
	w.bufferMu.Unlock()

	// 批量写入日志（串行写入，避免并发冲突）
	for _, entry := range entries {
		if entry.Channel != "" {
			logger := facades.Log().Channel(entry.Channel)
			switch entry.Level {
			case "debug":
				if len(entry.Args) > 0 {
					logger.Debugf(entry.Message, entry.Args...)
				} else {
					logger.Debug(entry.Message)
				}
			case "info":
				if len(entry.Args) > 0 {
					logger.Infof(entry.Message, entry.Args...)
				} else {
					logger.Info(entry.Message)
				}
			case "warning":
				if len(entry.Args) > 0 {
					logger.Warningf(entry.Message, entry.Args...)
				} else {
					logger.Warning(entry.Message)
				}
			case "error":
				if len(entry.Args) > 0 {
					logger.Errorf(entry.Message, entry.Args...)
				} else {
					logger.Error(entry.Message)
				}
			}
		} else {
			logger := facades.Log()
			switch entry.Level {
			case "debug":
				if len(entry.Args) > 0 {
					logger.Debugf(entry.Message, entry.Args...)
				} else {
					logger.Debug(entry.Message)
				}
			case "info":
				if len(entry.Args) > 0 {
					logger.Infof(entry.Message, entry.Args...)
				} else {
					logger.Info(entry.Message)
				}
			case "warning":
				if len(entry.Args) > 0 {
					logger.Warningf(entry.Message, entry.Args...)
				} else {
					logger.Warning(entry.Message)
				}
			case "error":
				if len(entry.Args) > 0 {
					logger.Errorf(entry.Message, entry.Args...)
				} else {
					logger.Error(entry.Message)
				}
			}
		}
	}
}

// LogToChannel 记录日志到指定通道（使用队列）
func LogToChannel(channel, level, message string, args ...interface{}) {
	writer := GetLogWriter()
	writer.Enqueue(channel, level, message, args...)
}




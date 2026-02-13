package services

import (
	"sync"
	"time"

	"goravel/app/models"
	"goravel/app/repositories"

	"github.com/goravel/framework/facades"
)

// MetricBuffer 性能指标缓冲区管理器
type MetricBuffer struct {
	buffer    []*models.ServerMetric
	bufferMu  sync.Mutex
	batchSize int
	interval  time.Duration
	stopChan  chan struct{}
	wg        sync.WaitGroup
	repo      *repositories.ServerMetricRepository
}

var (
	globalMetricBuffer *MetricBuffer
	metricBufferOnce   sync.Once
)

// GetMetricBuffer 获取全局指标缓冲区管理器（单例）
func GetMetricBuffer() *MetricBuffer {
	metricBufferOnce.Do(func() {
		globalMetricBuffer = &MetricBuffer{
			buffer:    make([]*models.ServerMetric, 0, 100),
			batchSize: 50,                // 批量写入大小：50条
			interval:  1 * time.Second,   // 批量写入间隔：1秒
			stopChan:  make(chan struct{}),
			repo:      repositories.NewServerMetricRepository(),
		}
		globalMetricBuffer.Start()
		facades.Log().Infof("启动性能指标批量写入队列，批量大小: %d, 写入间隔: %v", globalMetricBuffer.batchSize, globalMetricBuffer.interval)
	})
	return globalMetricBuffer
}

// Start 启动指标缓冲区管理器
func (b *MetricBuffer) Start() {
	b.wg.Add(1)
	go b.flushLoop()
}

// Stop 停止指标缓冲区管理器
func (b *MetricBuffer) Stop() {
	close(b.stopChan)
	b.wg.Wait()
	// 刷新剩余指标
	b.flush()
	facades.Log().Info("性能指标批量写入队列已停止")
}

// Enqueue 将性能指标加入缓冲区
func (b *MetricBuffer) Enqueue(metric *models.ServerMetric) {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	b.buffer = append(b.buffer, metric)

	// 如果缓冲区达到批量大小，立即刷新
	if len(b.buffer) >= b.batchSize {
		go b.flush()
	}
}

// flushLoop 定期刷新指标到数据库
func (b *MetricBuffer) flushLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flush()
		case <-b.stopChan:
			return
		}
	}
}

// flush 刷新缓冲区到数据库
func (b *MetricBuffer) flush() {
	b.bufferMu.Lock()
	if len(b.buffer) == 0 {
		b.bufferMu.Unlock()
		return
	}

	// 复制缓冲区并清空
	metrics := make([]*models.ServerMetric, len(b.buffer))
	copy(metrics, b.buffer)
	b.buffer = b.buffer[:0]
	b.bufferMu.Unlock()

	// 批量写入数据库
	if err := b.repo.BatchCreate(metrics); err != nil {
		facades.Log().Errorf("批量写入性能指标失败: %v，数据量: %d", err, len(metrics))
		return
	}

	facades.Log().Debugf("成功批量写入性能指标: %d 条", len(metrics))
}

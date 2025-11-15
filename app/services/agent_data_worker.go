package services

import (
	"context"
	"sync"

	"github.com/goravel/framework/facades"
)

// AgentDataWorker 异步处理agent数据的worker池
type AgentDataWorker struct {
	jobQueue    chan DataJob
	workerCount int
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// DataJob 数据任务接口
type DataJob interface {
	Execute() error
}

// NewAgentDataWorker 创建新的worker池
func NewAgentDataWorker(workerCount int) *AgentDataWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentDataWorker{
		jobQueue:    make(chan DataJob, 1000), // 缓冲队列，避免阻塞
		workerCount: workerCount,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动worker池
func (w *AgentDataWorker) Start() {
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}
}

// Stop 停止worker池
func (w *AgentDataWorker) Stop() {
	w.cancel()
	close(w.jobQueue)
	w.wg.Wait()
}

// Enqueue 将任务加入队列
func (w *AgentDataWorker) Enqueue(job DataJob) {
	select {
	case w.jobQueue <- job:
		// 任务已加入队列
	case <-w.ctx.Done():
		// worker池已停止
	default:
		// 队列已满，记录警告但不阻塞
		facades.Log().Channel("websocket").Warning("数据任务队列已满，丢弃任务")
	}
}

// worker 工作协程
func (w *AgentDataWorker) worker(id int) {
	defer w.wg.Done()

	for {
		select {
		case job, ok := <-w.jobQueue:
			if !ok {
				// 队列已关闭
				return
			}
			if err := job.Execute(); err != nil {
				facades.Log().Channel("websocket").Errorf("Worker %d 执行任务失败: %v", id, err)
			}
		case <-w.ctx.Done():
			return
		}
	}
}

// ServerStatusCache 服务器状态缓存
type ServerStatusCache struct {
	cache map[string]string
	mu    sync.RWMutex
}

var statusCache = &ServerStatusCache{
	cache: make(map[string]string),
}

// GetStatus 获取服务器状态（优先从缓存读取）
func (c *ServerStatusCache) GetStatus(serverID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if status, ok := c.cache[serverID]; ok {
		return status
	}
	return ""
}

// SetStatus 设置服务器状态
func (c *ServerStatusCache) SetStatus(serverID string, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[serverID] = status
}

// UpdateStatusFromDB 从数据库更新状态
func (c *ServerStatusCache) UpdateStatusFromDB(serverID string) {
	go func() {
		var servers []map[string]interface{}
		err := facades.Orm().Query().Table("servers").
			Select("status").
			Where("id", serverID).
			Get(&servers)

		if err == nil && len(servers) > 0 {
			if status, ok := servers[0]["status"].(string); ok {
				c.SetStatus(serverID, status)
			}
		}
	}()
}

// GetGlobalDataWorker 获取全局数据worker
var globalDataWorker *AgentDataWorker
var workerOnce sync.Once

func GetGlobalDataWorker() *AgentDataWorker {
	workerOnce.Do(func() {
		workerCount := 10
		globalDataWorker = NewAgentDataWorker(workerCount)
		globalDataWorker.Start()
		facades.Log().Infof("启动Agent数据Worker池，Worker数量: %d", workerCount)
	})
	return globalDataWorker
}

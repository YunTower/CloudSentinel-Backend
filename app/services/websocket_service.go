package services

import (
	"context"
	"sync"

	ws "goravel/app/services/websocket"
)

// WebSocketService 管理所有 WebSocket 连接，持有全局唯一的连接管理器。
type WebSocketService struct {
	manager ws.ConnectionManager
	ctx     context.Context
	cancel  context.CancelFunc
}

var (
	wsService     *WebSocketService
	wsServiceOnce sync.Once
)

// GetWebSocketService 获取 WebSocket 服务单例
func GetWebSocketService() *WebSocketService {
	wsServiceOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		alertSvc := NewAlertService()
		incidentSvc := NewIncidentService()
		wsService = &WebSocketService{
			manager: ws.NewConnectionManager(
				ws.WithServerStatusNotifier(func(serverID string, isOnline bool) {
					if isOnline {
						incidentSvc.ResolveServerOfflineIncident(serverID)
						alertSvc.NotifyServerOnline(serverID)
					} else {
						incidentSvc.OpenServerOfflineIncident(serverID)
						alertSvc.NotifyServerOffline(serverID)
					}
				}),
			),
			ctx:    ctx,
			cancel: cancel,
		}
		// 启动心跳检测
		go wsService.manager.StartHeartbeatChecker(ctx)
	})
	return wsService
}

// GetManager 获取连接管理器
func (s *WebSocketService) GetManager() ws.ConnectionManager {
	return s.manager
}

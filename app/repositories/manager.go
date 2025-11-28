package repositories

import "sync"

var (
	systemSettingRepoOnce     sync.Once
	serverRepoOnce            sync.Once
	serverMetricRepoOnce      sync.Once
	alertNotificationRepoOnce sync.Once

	systemSettingRepoInstance     *SystemSettingRepository
	serverRepoInstance            *ServerRepository
	serverMetricRepoInstance      *ServerMetricRepository
	alertNotificationRepoInstance *AlertNotificationRepository
)

// GetSystemSettingRepository 获取系统设置 Repository 单例
func GetSystemSettingRepository() *SystemSettingRepository {
	systemSettingRepoOnce.Do(func() {
		systemSettingRepoInstance = &SystemSettingRepository{}
	})
	return systemSettingRepoInstance
}

// GetServerRepository 获取服务器 Repository 单例
func GetServerRepository() *ServerRepository {
	serverRepoOnce.Do(func() {
		serverRepoInstance = &ServerRepository{}
	})
	return serverRepoInstance
}

// GetServerMetricRepository 获取服务器指标 Repository 单例
func GetServerMetricRepository() *ServerMetricRepository {
	serverMetricRepoOnce.Do(func() {
		serverMetricRepoInstance = &ServerMetricRepository{}
	})
	return serverMetricRepoInstance
}

// GetAlertNotificationRepository 获取告警通知 Repository 单例
func GetAlertNotificationRepository() *AlertNotificationRepository {
	alertNotificationRepoOnce.Do(func() {
		alertNotificationRepoInstance = &AlertNotificationRepository{}
	})
	return alertNotificationRepoInstance
}


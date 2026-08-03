package bootstrap

import (
	"sync"

	"github.com/goravel/framework/contracts/foundation"

	"goravel/app/facades"
	"goravel/app/services"
	"goravel/app/utils"
)

func Runners() []foundation.Runner {
	return []foundation.Runner{
		NewApplicationServicesRunner(),
	}
}

// ApplicationServicesRunner 统一管理 CloudSentinel 常驻后台服务的启动与优雅关闭。
type ApplicationServicesRunner struct {
	cleanupService     *services.CleanupService
	dataWorker         *services.AgentDataWorker
	logWriter          *utils.LogWriter
	metricBuffer       *services.MetricBuffer
	monitorService     *services.ServiceMonitorService
	stopLogLockCleanup func()
	started            chan struct{}
	done               chan struct{}
	startedOnce        sync.Once
	shutdownOnce       sync.Once
}

func NewApplicationServicesRunner() *ApplicationServicesRunner {
	return &ApplicationServicesRunner{
		cleanupService: services.NewCleanupService(),
		started:        make(chan struct{}),
		done:           make(chan struct{}),
	}
}

func (r *ApplicationServicesRunner) Signature() string {
	return "cloudsentinel:services"
}

func (r *ApplicationServicesRunner) ShouldRun() bool {
	return true
}

func (r *ApplicationServicesRunner) Run() error {
	defer r.startedOnce.Do(func() { close(r.started) })

	_ = services.CleanupStaleLogLocks()
	r.stopLogLockCleanup = services.StartPeriodicLogLockCleanup()
	services.SyncAuthSettingsFromDB()
	services.EnsureTLSCertificates()
	services.CheckTLSCertificateExpiry()

	r.dataWorker = services.GetGlobalDataWorker()
	r.logWriter = services.GetLogWriter()
	r.metricBuffer = services.GetMetricBuffer()
	r.monitorService = services.GetServiceMonitorService()

	go r.cleanupService.Start()
	r.monitorService.StartAll()

	r.startedOnce.Do(func() { close(r.started) })
	facades.Log().Info("CloudSentinel 后台服务已启动")
	<-r.done
	return nil
}

func (r *ApplicationServicesRunner) Shutdown() error {
	<-r.started
	r.shutdownOnce.Do(func() {
		if r.monitorService != nil {
			r.monitorService.StopAll()
		}
		if r.stopLogLockCleanup != nil {
			r.stopLogLockCleanup()
		}
		r.cleanupService.Stop()
		if r.metricBuffer != nil {
			r.metricBuffer.Stop()
		}
		if r.dataWorker != nil {
			r.dataWorker.Stop()
		}
		if r.logWriter != nil {
			r.logWriter.Stop()
		}
		close(r.done)
		facades.Log().Info("CloudSentinel 后台服务已停止")
	})
	return nil
}

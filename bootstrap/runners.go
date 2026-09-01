package bootstrap

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/route"

	"goravel/app/facades"
	"goravel/app/services"
	"goravel/app/utils"
)

func Runners() []foundation.Runner {
	return []foundation.Runner{
		NewApplicationServicesRunner(),
		NewPublicHTTPRunner(publicHTTPRoute),
	}
}

// PublicHTTPRunner 将公开站点作为第二个 HTTP 监听器纳入应用统一生命周期。
type PublicHTTPRunner struct {
	route  route.Route
	config publicHTTPConfig
}

func NewPublicHTTPRunner(publicRoute route.Route) *PublicHTTPRunner {
	certFile := facades.Config().GetString("http.public.tls.cert")
	keyFile := facades.Config().GetString("http.public.tls.key")
	if certFile == "" && keyFile == "" {
		// 默认复用管理监听器证书；需要不同域名证书时再通过 PUBLIC_TLS_* 覆盖。
		certFile = facades.Config().GetString("http.tls.ssl.cert")
		keyFile = facades.Config().GetString("http.tls.ssl.key")
	}

	return newPublicHTTPRunner(publicRoute, publicHTTPConfig{
		enabled:  facades.Config().GetBool("http.public.enabled"),
		host:     facades.Config().GetString("http.public.host"),
		port:     facades.Config().GetString("http.public.port"),
		certFile: certFile,
		keyFile:  keyFile,
	})
}

type publicHTTPConfig struct {
	enabled  bool
	host     string
	port     string
	certFile string
	keyFile  string
}

func newPublicHTTPRunner(publicRoute route.Route, config publicHTTPConfig) *PublicHTTPRunner {
	return &PublicHTTPRunner{route: publicRoute, config: config}
}

func (r *PublicHTTPRunner) Signature() string {
	return "cloudsentinel:http-public"
}

func (r *PublicHTTPRunner) ShouldRun() bool {
	return r.route != nil && r.config.enabled
}

func (r *PublicHTTPRunner) Run() error {
	if r.config.host == "" || r.config.port == "" {
		return fmt.Errorf("PUBLIC_HOST and PUBLIC_PORT must be configured when public HTTP is enabled")
	}

	address := net.JoinHostPort(r.config.host, r.config.port)
	if (r.config.certFile == "") != (r.config.keyFile == "") {
		return fmt.Errorf("PUBLIC_TLS_CERT_FILE and PUBLIC_TLS_KEY_FILE must be configured together")
	}
	if r.config.certFile != "" {
		return r.route.RunTLSWithCert(address, r.config.certFile, r.config.keyFile)
	}
	return r.route.Run(address)
}

func (r *PublicHTTPRunner) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return r.route.Shutdown(ctx)
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
	r.logWriter = utils.GetLogWriter()
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

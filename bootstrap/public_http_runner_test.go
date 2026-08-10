package bootstrap

import (
	"context"
	"testing"

	"github.com/goravel/framework/contracts/route"
)

type publicRouteRecorder struct {
	route.Route
	runAddress string
	tlsAddress string
	certFile   string
	keyFile    string
	shutdown   bool
}

func (r *publicRouteRecorder) Run(host ...string) error {
	r.runAddress = host[0]
	return nil
}

func (r *publicRouteRecorder) RunTLSWithCert(host, certFile, keyFile string) error {
	r.tlsAddress = host
	r.certFile = certFile
	r.keyFile = keyFile
	return nil
}

func (r *publicRouteRecorder) Shutdown(ctx ...context.Context) error {
	r.shutdown = len(ctx) == 1
	return nil
}

func TestPublicHTTPRunner_按配置启动独立HTTP监听器(t *testing.T) {
	recorder := &publicRouteRecorder{}
	runner := newPublicHTTPRunner(recorder, publicHTTPConfig{
		enabled: true,
		host:    "::1",
		port:    "3001",
	})

	if !runner.ShouldRun() {
		t.Fatal("启用公开监听器后 ShouldRun 应为 true")
	}
	if err := runner.Run(); err != nil {
		t.Fatalf("启动公开监听器失败: %v", err)
	}
	if recorder.runAddress != "[::1]:3001" {
		t.Fatalf("监听地址异常: %q", recorder.runAddress)
	}
	if err := runner.Shutdown(); err != nil || !recorder.shutdown {
		t.Fatalf("公开监听器未进入统一关闭流程: err=%v shutdown=%v", err, recorder.shutdown)
	}
}

func TestPublicHTTPRunner_配置证书时启动TLS(t *testing.T) {
	recorder := &publicRouteRecorder{}
	runner := newPublicHTTPRunner(recorder, publicHTTPConfig{
		enabled:  true,
		host:     "0.0.0.0",
		port:     "3001",
		certFile: "public.crt",
		keyFile:  "public.key",
	})

	if err := runner.Run(); err != nil {
		t.Fatalf("启动公开 TLS 监听器失败: %v", err)
	}
	if recorder.tlsAddress != "0.0.0.0:3001" || recorder.certFile != "public.crt" || recorder.keyFile != "public.key" {
		t.Fatalf("TLS 参数异常: address=%q cert=%q key=%q", recorder.tlsAddress, recorder.certFile, recorder.keyFile)
	}
}

func TestPublicHTTPRunner_拒绝不完整配置(t *testing.T) {
	cases := []publicHTTPConfig{
		{enabled: true, port: "3001"},
		{enabled: true, host: "0.0.0.0"},
		{enabled: true, host: "0.0.0.0", port: "3001", certFile: "public.crt"},
	}
	for _, config := range cases {
		runner := newPublicHTTPRunner(&publicRouteRecorder{}, config)
		if err := runner.Run(); err == nil {
			t.Fatalf("不完整配置应启动失败: %+v", config)
		}
	}
}

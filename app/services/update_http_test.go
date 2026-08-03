package services

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	contractsclient "github.com/goravel/framework/contracts/http/client"
	frameworkjson "github.com/goravel/framework/foundation/json"
	frameworkclient "github.com/goravel/framework/http/client"
)

func newUpdateHTTPFactory(t *testing.T) *frameworkclient.Factory {
	t.Helper()

	factory, err := frameworkclient.NewFactory(&frameworkclient.FactoryConfig{
		Default: "github",
		Clients: map[string]frameworkclient.Config{
			"github": {
				BaseUrl: "https://api.github.test",
				Timeout: time.Second,
			},
			"download": {
				Timeout: time.Second,
			},
		},
	}, nil, frameworkjson.New(), nil)
	if err != nil {
		t.Fatalf("创建 HTTP 测试工厂失败: %v", err)
	}

	return factory
}

func TestFetchLatestReleaseUsesTypedResponse(t *testing.T) {
	factory := newUpdateHTTPFactory(t)
	url := "https://api.github.test/repos/YunTower/CloudSentinel/releases/latest"
	factory.Fake(map[string]any{
		url: factory.Response().Json(http.StatusOK, map[string]any{
			"tag_name":   "v1.17.2-beta.1",
			"created_at": "2026-08-01T12:00:00Z",
			"body":       "变更说明",
			"assets": []map[string]any{
				{
					"name":                 "dashboard-linux-amd64.tar.gz",
					"browser_download_url": "https://downloads.test/dashboard-linux-amd64.tar.gz",
				},
			},
		}),
	}).PreventStrayRequests()

	release, err := newUpdateService(factory).FetchLatestRelease(url)
	if err != nil {
		t.Fatalf("获取最新版本失败: %v", err)
	}
	if release.TagName != "v1.17.2-beta.1" || release.NormalizedTagName != "1.17.2-beta.1" {
		t.Fatalf("版本号绑定错误: %+v", release)
	}
	if release.VersionType != "beta" || release.CreatedAt != "2026-08-01T12:00:00Z" || release.Body != "变更说明" {
		t.Fatalf("版本元数据绑定错误: %+v", release)
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "dashboard-linux-amd64.tar.gz" {
		t.Fatalf("发布文件绑定错误: %+v", release.Assets)
	}
	if !factory.AssertSent(func(request contractsclient.Request) bool {
		return request.ClientName() == "github" && request.Header("Accept") == "application/json"
	}) {
		t.Fatal("版本请求应使用 github 命名客户端并声明 JSON 响应")
	}
}

func TestFetchLatestReleaseByChannel(t *testing.T) {
	factory := newUpdateHTTPFactory(t)
	url := "https://api.github.test/repos/YunTower/CloudSentinel/releases"
	factory.Fake(map[string]any{
		url: factory.Response().Json(http.StatusOK, []map[string]any{
			{"tag_name": "v1.18.0-dev.1"},
			{"tag_name": "v1.17.2-beta.2", "body": "测试版"},
			{"tag_name": "v1.17.1", "body": "正式版"},
		}),
	}).PreventStrayRequests()

	service := newUpdateService(factory)
	beta, err := service.FetchLatestReleaseByChannel(url, "beta")
	if err != nil {
		t.Fatalf("获取测试版失败: %v", err)
	}
	if beta.TagName != "v1.17.2-beta.2" || beta.Body != "测试版" {
		t.Fatalf("测试版渠道选择错误: %+v", beta)
	}

	// Fake 响应体是一次性流，为第二次请求重新注册同一份响应。
	factory.Fake(map[string]any{
		url: factory.Response().Json(http.StatusOK, []map[string]any{
			{"tag_name": "v1.18.0-dev.1"},
			{"tag_name": "v1.17.2-beta.2"},
			{"tag_name": "v1.17.1", "body": "正式版"},
		}),
	}).PreventStrayRequests()
	release, err := service.FetchLatestReleaseByChannel(url, "release")
	if err != nil {
		t.Fatalf("获取正式版失败: %v", err)
	}
	if release.TagName != "v1.17.1" || release.VersionType != "release" {
		t.Fatalf("正式版渠道选择错误: %+v", release)
	}
}

func TestDownloadFileStreamsResponse(t *testing.T) {
	factory := newUpdateHTTPFactory(t)
	url := "https://downloads.test/dashboard-linux-amd64.tar.gz"
	payload := strings.Repeat("CloudSentinel", 10_000)
	factory.Fake(map[string]any{
		url: factory.Response().Make(http.StatusOK, payload, http.Header{
			"Content-Length": []string{"130000"},
		}),
	}).PreventStrayRequests()

	target := filepath.Join(t.TempDir(), "downloads", "release.tar.gz")
	var progress []int
	if err := newUpdateService(factory).DownloadFile(url, target, func(value int) {
		progress = append(progress, value)
	}); err != nil {
		t.Fatalf("流式下载失败: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("读取下载文件失败: %v", err)
	}
	if string(content) != payload {
		t.Fatalf("下载内容不一致: got=%d want=%d", len(content), len(payload))
	}
	if !slices.Contains(progress, 100) {
		t.Fatalf("下载完成时应回调 100%% 进度: %v", progress)
	}
	if !factory.AssertSent(func(request contractsclient.Request) bool {
		return request.ClientName() == "download" && request.Url() == url
	}) {
		t.Fatal("下载请求应使用 download 命名客户端")
	}
}

func TestFetchLatestReleaseRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name      string
		response  func(*frameworkclient.Factory) any
		wantError string
	}{
		{
			name: "未找到",
			response: func(factory *frameworkclient.Factory) any {
				return factory.Response().Status(http.StatusNotFound)
			},
			wantError: "未找到最新的版本信息",
		},
		{
			name: "缺少版本号",
			response: func(factory *frameworkclient.Factory) any {
				return factory.Response().Json(http.StatusOK, map[string]any{"body": "invalid"})
			},
			wantError: "tag_name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factory := newUpdateHTTPFactory(t)
			url := "https://api.github.test/releases/latest"
			factory.Fake(map[string]any{url: test.response(factory)}).PreventStrayRequests()

			_, err := newUpdateService(factory).FetchLatestRelease(url)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("错误不符合预期: got=%v want~=%q", err, test.wantError)
			}
		})
	}
}

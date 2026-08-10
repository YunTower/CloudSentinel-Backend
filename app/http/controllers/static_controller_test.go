package controllers

import (
	"errors"
	"testing"
	"testing/fstest"
)

func TestStaticController_隔离管理端与公开端资源(t *testing.T) {
	adminFiles := fstest.MapFS{
		"public/admin/index.html": &fstest.MapFile{Data: []byte("admin")},
		"public/admin/js/app.js":  &fstest.MapFile{Data: []byte("admin-js")},
	}
	publicFiles := fstest.MapFS{
		"public/public/index.html": &fstest.MapFile{Data: []byte("public")},
		"public/public/js/app.js":  &fstest.MapFile{Data: []byte("public-js")},
	}

	adminSite := NewStaticController(adminFiles, AdminAssetsRoot)
	publicSite := NewStaticController(publicFiles, PublicAssetsRoot)

	adminIndex, _, err := adminSite.resolve("/")
	if err != nil || string(adminIndex) != "admin" {
		t.Fatalf("管理端首页解析失败: data=%q err=%v", adminIndex, err)
	}
	publicIndex, _, err := publicSite.resolve("/")
	if err != nil || string(publicIndex) != "public" {
		t.Fatalf("公开端首页解析失败: data=%q err=%v", publicIndex, err)
	}

	_, _, err = publicSite.resolve("/../admin/js/app.js")
	if !errors.Is(err, errStaticFileNotFound) {
		t.Fatalf("公开端不应读取管理端资源，实际错误: %v", err)
	}
}

func TestStaticController_SPA导航回退且资源缺失返回404(t *testing.T) {
	files := fstest.MapFS{
		"public/public/index.html": &fstest.MapFile{Data: []byte("public-index")},
	}
	site := NewStaticController(files, PublicAssetsRoot)

	data, contentType, err := site.resolve("/public/status")
	if err != nil {
		t.Fatalf("SPA 导航应回退首页: %v", err)
	}
	if string(data) != "public-index" || contentType != "text/html; charset=utf-8" {
		t.Fatalf("SPA 回退结果异常: data=%q contentType=%q", data, contentType)
	}

	_, _, err = site.resolve("/js/missing.js")
	if !errors.Is(err, errStaticFileNotFound) {
		t.Fatalf("缺失的静态资源应返回 not found，实际错误: %v", err)
	}
}

func TestStaticController_后端路径不进入SPAFallback(t *testing.T) {
	paths := []string{"/api", "/api/settings/public", "/ws", "/ws/frontend"}
	for _, requestPath := range paths {
		if !isBackendPath(requestPath) {
			t.Fatalf("后端路径 %q 应被静态站点拒绝", requestPath)
		}
	}
}

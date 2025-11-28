package controllers

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"

	"goravel/app/utils"

	goravelhttp "github.com/goravel/framework/contracts/http"
)

var (
	PublicFiles embed.FS
)

type StaticController struct {
}

func NewStaticController() *StaticController {
	return &StaticController{}
}

// ServeStatic 提供静态文件服务
func (r *StaticController) ServeStatic(ctx goravelhttp.Context) goravelhttp.Response {
	path := ctx.Request().Path()

	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
		return utils.ErrorResponse(ctx, http.StatusNotFound, "Not found")
	}

	// 移除前导斜杠
	path = strings.TrimPrefix(path, "/")

	// 如果路径为空，默认为 index.html
	if path == "" {
		path = "index.html"
	}

	// 检查 PublicFiles 是否已初始化并包含文件
	entries, listErr := PublicFiles.ReadDir("public")
	if listErr != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "Static files not embedded. PublicFiles not initialized. Please rebuild the application.")
	}
	if len(entries) == 0 {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "Static files not embedded. Public directory was empty during compilation. Please build frontend first (pnpm run build) and rebuild the backend.")
	}

	fsPath := "public/" + strings.ReplaceAll(path, "\\", "/")

	// 尝试读取文件
	data, err := PublicFiles.ReadFile(fsPath)
	if err != nil {
		// 如果文件不存在，检查路径是否有文件扩展名
		ext := strings.ToLower(filepath.Ext(path))

		if ext != "" && ext != ".html" {
			return utils.ErrorResponse(ctx, http.StatusNotFound, "File not found")
		}

		indexData, indexErr := PublicFiles.ReadFile("public/index.html")
		if indexErr != nil {
			entries, listErr := PublicFiles.ReadDir("public")
			if listErr != nil {
				return utils.ErrorResponse(ctx, http.StatusInternalServerError, "Embedded files not available. Please ensure frontend is built and rebuild the backend.")
			}
			if len(entries) == 0 {
				return utils.ErrorResponse(ctx, http.StatusInternalServerError, "Public directory is empty. Please build frontend first.")
			}
			return utils.ErrorResponse(ctx, http.StatusNotFound, "index.html not found in embedded files")
		}

		return ctx.Response().Header("Content-Type", "text/html; charset=utf-8").String(http.StatusOK, string(indexData))
	}

	// 根据文件扩展名设置 Content-Type
	contentType := getContentType(path)

	return ctx.Response().Header("Content-Type", contentType).String(http.StatusOK, string(data))
}

// getContentType 根据文件扩展名返回 Content-Type
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}

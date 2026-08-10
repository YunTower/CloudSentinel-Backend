package controllers

import (
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"goravel/app/utils"

	goravelhttp "github.com/goravel/framework/contracts/http"
)

var (
	AdminFiles  fs.FS
	PublicFiles fs.FS
)

const (
	AdminAssetsRoot  = "public/admin"
	PublicAssetsRoot = "public/public"
)

var (
	errStaticFilesUnavailable = errors.New("static files unavailable")
	errStaticFileNotFound     = errors.New("static file not found")
)

type StaticController struct {
	files fs.FS
	root  string
}

func NewStaticController(files fs.FS, root string) *StaticController {
	return &StaticController{files: files, root: strings.Trim(root, "/")}
}

func (r *StaticController) Available() bool {
	if r.files == nil || r.root == "" {
		return false
	}
	entries, err := fs.ReadDir(r.files, r.root)
	return err == nil && len(entries) > 0
}

// ServeStatic 提供静态文件服务
func (r *StaticController) ServeStatic(ctx goravelhttp.Context) goravelhttp.Response {
	path := ctx.Request().Path()

	if isBackendPath(path) {
		return utils.ErrorResponse(ctx, http.StatusNotFound, "Not found")
	}

	data, contentType, err := r.resolve(path)
	if errors.Is(err, errStaticFileNotFound) {
		return utils.ErrorResponse(ctx, http.StatusNotFound, "File not found")
	}
	if err != nil {
		return utils.ErrorResponse(ctx, http.StatusInternalServerError, "Static files not embedded. Please build both frontends and rebuild the backend.")
	}

	return ctx.Response().Data(http.StatusOK, contentType, data)
}

func (r *StaticController) resolve(requestPath string) ([]byte, string, error) {
	if !r.Available() {
		return nil, "", errStaticFilesUnavailable
	}

	assetPath := cleanAssetPath(requestPath)
	data, err := fs.ReadFile(r.files, path.Join(r.root, assetPath))
	if err == nil {
		return data, getContentType(assetPath), nil
	}

	ext := strings.ToLower(path.Ext(assetPath))
	if ext != "" && ext != ".html" {
		return nil, "", errStaticFileNotFound
	}

	data, err = fs.ReadFile(r.files, path.Join(r.root, "index.html"))
	if err != nil {
		return nil, "", errStaticFilesUnavailable
	}
	return data, "text/html; charset=utf-8", nil
}

func cleanAssetPath(requestPath string) string {
	cleaned := path.Clean("/" + strings.ReplaceAll(requestPath, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "index.html"
	}
	return cleaned
}

func isBackendPath(requestPath string) bool {
	cleaned := "/" + strings.Trim(strings.ReplaceAll(requestPath, "\\", "/"), "/")
	return cleaned == "/api" || strings.HasPrefix(cleaned, "/api/") ||
		cleaned == "/ws" || strings.HasPrefix(cleaned, "/ws/")
}

// getContentType 根据文件扩展名返回 Content-Type
func getContentType(assetPath string) string {
	ext := strings.ToLower(path.Ext(strings.ReplaceAll(assetPath, "\\", "/")))

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

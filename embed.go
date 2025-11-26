package main

import (
	"embed"

	"goravel/app/http/controllers"
)

//go:embed public
var publicFiles embed.FS

func init() {
	// 将嵌入的文件系统传递给控制器
	controllers.PublicFiles = publicFiles
}

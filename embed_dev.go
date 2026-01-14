//go:build !production

package main

import (
	"embed"
	"fmt"

	"goravel/app/http/controllers"
	"goravel/app/services"
)

//go:embed resources
var resourceFiles embed.FS

func init() {
	controllers.PublicFiles = embed.FS{}

	// 验证 resourceFiles 是否包含文件
	resourceEntries, resourceErr := resourceFiles.ReadDir("resources")
	if resourceErr != nil {
		panic(fmt.Sprintf("Failed to read embedded resources directory: %v", resourceErr))
	}
	if len(resourceEntries) == 0 {
		panic("Embedded resources directory is empty.")
	}

	services.ResourceFiles = resourceFiles
}

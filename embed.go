//go:build production

package main

import (
	"embed"
	"fmt"

	"goravel/app/http/controllers"
	"goravel/app/services"
)

//go:embed public
var publicFiles embed.FS

//go:embed resources
var resourceFiles embed.FS

func init() {
	// 验证 publicFiles 是否包含文件
	entries, err := publicFiles.ReadDir("public")
	if err != nil {
		panic(fmt.Sprintf("Failed to read embedded public directory: %v. Make sure frontend is built before compiling backend.", err))
	}
	if len(entries) == 0 {
		panic("Embedded public directory is empty. Please build frontend (pnpm run build) before compiling backend.")
	}

	// 验证 resourceFiles 是否包含文件
	resourceEntries, resourceErr := resourceFiles.ReadDir("resources")
	if resourceErr != nil {
		panic(fmt.Sprintf("Failed to read embedded resources directory: %v", resourceErr))
	}
	if len(resourceEntries) == 0 {
		panic("Embedded resources directory is empty.")
	}

	controllers.PublicFiles = publicFiles
	services.ResourceFiles = resourceFiles

	testEntries, testErr := controllers.PublicFiles.ReadDir("public")
	if testErr != nil || len(testEntries) == 0 {
		panic(fmt.Sprintf("PublicFiles assignment failed or is empty. Error: %v, Entries: %d", testErr, len(testEntries)))
	}
}

//go:build production

package main

import (
	"embed"
	"fmt"

	"goravel/app/http/controllers"
)

//go:embed public/admin
var adminFiles embed.FS

//go:embed public/public
var publicFiles embed.FS

func init() {
	validateEmbeddedSite(adminFiles, controllers.AdminAssetsRoot, "admin")
	validateEmbeddedSite(publicFiles, controllers.PublicAssetsRoot, "public")

	controllers.AdminFiles = adminFiles
	controllers.PublicFiles = publicFiles
}

func validateEmbeddedSite(files embed.FS, root, name string) {
	entries, err := files.ReadDir(root)
	if err != nil {
		panic(fmt.Sprintf("Failed to read embedded %s directory: %v. Make sure both frontends are built before compiling backend.", name, err))
	}
	if len(entries) == 0 {
		panic(fmt.Sprintf("Embedded %s directory is empty. Please build frontend (pnpm run build) before compiling backend.", name))
	}
	if _, err := files.ReadFile(root + "/index.html"); err != nil {
		panic(fmt.Sprintf("Embedded %s index.html is missing: %v", name, err))
	}
}

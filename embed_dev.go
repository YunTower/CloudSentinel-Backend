//go:build !production

package main

import (
	"embed"

	"goravel/app/http/controllers"
)

func init() {
	controllers.AdminFiles = embed.FS{}
	controllers.PublicFiles = embed.FS{}
}

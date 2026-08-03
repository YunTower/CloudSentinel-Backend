package bootstrap

import (
	contractsfoundation "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/foundation"

	"goravel/config"
)

func Boot() contractsfoundation.Application {
	return foundation.Setup().
		WithProviders(Providers).
		WithCommands(Commands).
		WithCommandsFilter(CommandsFilter).
		WithMiddleware(Middleware).
		WithRouting(Routing).
		WithSchedule(Schedule).
		WithMigrations(Migrations).
		WithSeeders(Seeders).
		WithJobs(Jobs).
		WithRunners(Runners).
		WithConfig(config.Boot).
		Create()
}

package providers

import (
	"goravel/app/console"

	"github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/facades"
)

// ScheduleServiceProvider registers application schedules in every runtime
// environment. ConsoleServiceProvider is intentionally omitted in production,
// so schedules must not depend on the Artisan command provider.
type ScheduleServiceProvider struct {
}

func (receiver *ScheduleServiceProvider) Register(app foundation.Application) {
}

func (receiver *ScheduleServiceProvider) Boot(app foundation.Application) {
	kernel := console.Kernel{}
	facades.Schedule().Register(kernel.Schedule())
}

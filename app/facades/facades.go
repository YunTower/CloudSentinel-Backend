package facades

import (
	"github.com/goravel/framework/contracts/auth"
	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/config"
	"github.com/goravel/framework/contracts/console"
	"github.com/goravel/framework/contracts/database/db"
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/database/seeder"
	foundationcontract "github.com/goravel/framework/contracts/foundation"
	"github.com/goravel/framework/contracts/hash"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/http/client"
	"github.com/goravel/framework/contracts/log"
	"github.com/goravel/framework/contracts/process"
	"github.com/goravel/framework/contracts/queue"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/contracts/schedule"
	"github.com/goravel/framework/contracts/validation"
	"github.com/goravel/framework/errors"
	"github.com/goravel/framework/foundation"
)

func App() foundationcontract.Application {
	if foundation.App == nil {
		panic(errors.ApplicationNotSet.SetModule(errors.ModuleFacade))
	}
	return foundation.App
}

func Artisan() console.Artisan           { return App().MakeArtisan() }
func Auth(ctx ...http.Context) auth.Auth { return App().MakeAuth(ctx...) }
func Cache() cache.Cache                 { return App().MakeCache() }
func Config() config.Config              { return App().MakeConfig() }
func DB() db.DB                          { return App().MakeDB() }
func Hash() hash.Hash                    { return App().MakeHash() }
func Http() client.Factory               { return App().MakeHttp() }
func Log() log.Log                       { return App().MakeLog() }
func Orm() orm.Orm                       { return App().MakeOrm() }
func Process() process.Process           { return App().MakeProcess() }
func Queue() queue.Queue                 { return App().MakeQueue() }
func Route() route.Route                 { return App().MakeRoute() }
func Schedule() schedule.Schedule        { return App().MakeSchedule() }
func Schema() schema.Schema              { return App().MakeSchema() }
func Seeder() seeder.Facade              { return App().MakeSeeder() }
func Validation() validation.Validation  { return App().MakeValidation() }

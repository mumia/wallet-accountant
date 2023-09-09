package main

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/api"
	"walletaccountant/clock"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
)

func main() {
	fx.New(
		fx.Provide(
			fx.Annotate(api.NewServer, fx.ParamTags(`group:"routes"`, `group:"aggregateFactories"`)),
			zap.NewDevelopment,
			clock.NewClock,

			// Routes
			api.AsRoute(account.NewRegisterApi),

			// Mediator
			account.NewMediator,

			// Repositories
			//account.NewAccountRepository,

			// Aggregate factory
			definitions.AsAggregateFactory(account.NewFactory),

			// Event data registers
			definitions.AsEventDataRegister(account.NewEventRegister),

			// Event Store DB
			eventstoredb.NewClient,
			fx.Annotate(eventstoredb.NewEventStoreFactory, fx.ParamTags(`group:"eventDataRegisters"`)),

			// Event horizon stuff
			fx.Annotate(bus.NewCommandHandler, fx.As(new(eventhorizon.CommandHandler))),
		),
		fx.Invoke(func(engine *gin.Engine) { /* Nothing to do here */ }),
		// Command handler
		fx.Invoke(account.RegisterCommandHandler),
	).Run()
}

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
	"walletaccountant/commandapis"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/mongodb"
	"walletaccountant/projector"
	"walletaccountant/queryapis"
	"walletaccountant/tagcategory"
)

func main() {
	fx.New(
		fx.Provide(
			fx.Annotate(api.NewServer, fx.ParamTags(`group:"routes"`, `group:"aggregateFactories"`)),
			zap.NewDevelopment,
			clock.NewClock,

			// Command routes
			definitions.AsRoute(commandapis.NewRegisterNewAccountApi),
			definitions.AsRoute(commandapis.NewNewTagAndCategoryApi),
			definitions.AsRoute(commandapis.NewNewTagApi),

			// Query routes
			definitions.AsRoute(queryapis.NewReadAllAccountsApi),
			definitions.AsRoute(queryapis.NewReadAccountsApi),
			definitions.AsRoute(queryapis.NewReadAllTagsApi),

			// CommandMediator
			fx.Annotate(account.NewCommandMediator, fx.As(new(account.CommandMediatorer))),
			fx.Annotate(tagcategory.NewCommandMediator, fx.As(new(tagcategory.CommandMediatorer))),

			// QueryMediator
			fx.Annotate(account.NewQueryMediator, fx.As(new(account.QueryMediatorer))),
			fx.Annotate(tagcategory.NewQueryMediator, fx.As(new(tagcategory.QueryMediatorer))),

			// Aggregate factory
			definitions.AsAggregateFactory(account.NewFactory),
			definitions.AsAggregateFactory(tagcategory.NewFactory),

			// Event data registers
			definitions.AsEventDataRegister(account.NewEventRegister),
			definitions.AsEventDataRegister(tagcategory.NewEventRegister),

			// Event Store DB
			fx.Annotate(eventstoredb.NewClient, fx.As(new(eventstoredb.EventStorerer))),
			fx.Annotate(eventstoredb.NewEventStoreFactory, fx.As(new(eventstoredb.EventStoreCreator))),
			fx.Annotate(eventstoredb.NewIdCreator, fx.As(new(eventstoredb.IdGenerator))),

			// Mongo DB
			mongodb.NewMongoClient,

			// Projections
			fx.Annotate(
				projector.NewEventMatcherHandlerRegistry,
				fx.ParamTags(`group:"eventMatcherHandleProviders"`),
			),
			definitions.AsEventMatcherHandleProvider(account.NewProjectionConfig),
			fx.Annotate(account.NewProjection, fx.As(new(account.ReadModelProjection))),
			definitions.AsEventMatcherHandleProvider(tagcategory.NewProjectionConfig),
			fx.Annotate(tagcategory.NewProjection, fx.As(new(tagcategory.ReadModelProjection))),

			// Read model repositories
			fx.Annotate(account.NewReadModelRepository, fx.As(new(account.ReadModeler))),
			fx.Annotate(tagcategory.NewReadModelRepository, fx.As(new(tagcategory.ReadModeler))),

			// Event horizon stuff
			fx.Annotate(bus.NewCommandHandler, fx.As(new(eventhorizon.CommandHandler))),
		),
		// Start api server
		fx.Invoke(func(engine *gin.Engine) { /* Nothing to do here */ }),

		// Command handler
		fx.Invoke(account.RegisterCommandHandler),
		fx.Invoke(tagcategory.RegisterCommandHandler),

		// Event stream subscriptions
		fx.Invoke(account.SubscribeEventStream),
		fx.Invoke(tagcategory.SubscribeEventStream),

		// Event registration
		fx.Invoke(fx.Annotate(eventstoredb.RegisterEvents, fx.ParamTags(`group:"eventDataRegisters"`))),

		//fx.WithLogger(
		//	func(log *zap.Logger) fxevent.Logger {
		//		return &fxevent.ZapLogger{Logger: log}
		//	},
		//),
	).Run()
}

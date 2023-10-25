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
	"walletaccountant/movementtype"
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
			definitions.AsRoute(commandapis.NewNewTagWithExistingCategoryApi),
			definitions.AsRoute(commandapis.NewRegisterNewMovementTypeApi),

			// Query routes
			definitions.AsRoute(queryapis.NewReadAllAccountsApi),
			definitions.AsRoute(queryapis.NewReadAccountsApi),
			definitions.AsRoute(queryapis.NewReadAllTagsApi),
			definitions.AsRoute(queryapis.NewReadAllMovementTypesApi),
			definitions.AsRoute(queryapis.NewReadMovementTypeApi),

			// CommandMediator
			fx.Annotate(account.NewCommandMediator, fx.As(new(account.CommandMediatorer))),
			fx.Annotate(tagcategory.NewCommandMediator, fx.As(new(tagcategory.CommandMediatorer))),
			fx.Annotate(movementtype.NewCommandMediator, fx.As(new(movementtype.CommandMediatorer))),

			// QueryMediator
			fx.Annotate(account.NewQueryMediator, fx.As(new(account.QueryMediatorer))),
			fx.Annotate(tagcategory.NewQueryMediator, fx.As(new(tagcategory.QueryMediatorer))),
			fx.Annotate(movementtype.NewQueryMediator, fx.As(new(movementtype.QueryMediatorer))),

			// Aggregate factory
			definitions.AsAggregateFactory(account.NewFactory),
			definitions.AsAggregateFactory(tagcategory.NewFactory),
			definitions.AsAggregateFactory(movementtype.NewFactory),

			// Event data registers
			definitions.AsEventDataRegister(account.NewEventRegister),
			definitions.AsEventDataRegister(tagcategory.NewEventRegister),
			definitions.AsEventDataRegister(movementtype.NewEventRegister),

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
			definitions.AsEventMatcherHandleProvider(movementtype.NewProjectionConfig),
			fx.Annotate(movementtype.NewProjection, fx.As(new(movementtype.ReadModelProjection))),

			// Read model repositories
			fx.Annotate(account.NewReadModelRepository, fx.As(new(account.ReadModeler))),
			fx.Annotate(tagcategory.NewReadModelRepository, fx.As(new(tagcategory.ReadModeler))),
			fx.Annotate(movementtype.NewReadModelRepository, fx.As(new(movementtype.ReadModeler))),

			// Event horizon stuff
			fx.Annotate(bus.NewCommandHandler, fx.As(new(eventhorizon.CommandHandler))),
		),
		// Start api server
		fx.Invoke(func(engine *gin.Engine) { /* Nothing to do here */ }),

		// Command handler
		fx.Invoke(account.RegisterCommandHandler),
		fx.Invoke(tagcategory.RegisterCommandHandler),
		fx.Invoke(movementtype.RegisterCommandHandler),

		// Event stream subscriptions
		fx.Invoke(account.SubscribeEventStream),
		fx.Invoke(tagcategory.SubscribeEventStream),
		fx.Invoke(movementtype.SubscribeEventStream),

		// Event registration
		fx.Invoke(fx.Annotate(eventstoredb.RegisterEvents, fx.ParamTags(`group:"eventDataRegisters"`))),

		//fx.WithLogger(
		//	func(log *zap.Logger) fxevent.Logger {
		//		return &fxevent.ZapLogger{Logger: log}
		//	},
		//),
	).Run()
}

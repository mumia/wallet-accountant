package main

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/api"
	"walletaccountant/clock"
	"walletaccountant/commandapis"
	"walletaccountant/definitions"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/mongodb"
	"walletaccountant/movementtype"
	"walletaccountant/queryapis"
	"walletaccountant/saga"
	"walletaccountant/tagcategory"
	"walletaccountant/websocket"
)

func main() {
	fx.New(
		fx.Provide(
			fx.Annotate(api.NewServer, fx.ParamTags(`group:"routes"`, `group:"aggregateFactories"`)),
			zap.NewDevelopment,
			clock.NewClock,
		),
		fx.Provide(
			// Command routes
			definitions.AsRoute(commandapis.NewRegisterNewAccountApi),
			definitions.AsRoute(commandapis.NewNewTagAndCategoryApi),
			definitions.AsRoute(commandapis.NewNewTagWithExistingCategoryApi),
			definitions.AsRoute(commandapis.NewRegisterNewMovementTypeApi),
			definitions.AsRoute(commandapis.NewAccountMonthRegisterNewMovementApi),
			definitions.AsRoute(commandapis.NewEndAccountMonthApi),
		),
		fx.Provide(
			// Query routes
			definitions.AsRoute(queryapis.NewReadAllAccountsApi),
			definitions.AsRoute(queryapis.NewReadAccountsApi),
			definitions.AsRoute(queryapis.NewReadAllTagsApi),
			definitions.AsRoute(queryapis.NewReadAllMovementTypesApi),
			definitions.AsRoute(queryapis.NewReadMovementTypeApi),
			definitions.AsRoute(queryapis.NewReadMovementTypeByAccountApi),
			definitions.AsRoute(queryapis.NewReadCurrentAccountMonthApi),
		),
		fx.Provide(
			// CommandMediator
			fx.Annotate(account.NewCommandMediator, fx.As(new(account.CommandMediatorer))),
			fx.Annotate(tagcategory.NewCommandMediator, fx.As(new(tagcategory.CommandMediatorer))),
			fx.Annotate(movementtype.NewCommandMediator, fx.As(new(movementtype.CommandMediatorer))),
			fx.Annotate(accountmonth.NewCommandMediator, fx.As(new(accountmonth.CommandMediatorer))),
		),
		fx.Provide(
			// QueryMediator
			fx.Annotate(account.NewQueryMediator, fx.As(new(account.QueryMediatorer))),
			fx.Annotate(tagcategory.NewQueryMediator, fx.As(new(tagcategory.QueryMediatorer))),
			fx.Annotate(movementtype.NewQueryMediator, fx.As(new(movementtype.QueryMediatorer))),
			fx.Annotate(accountmonth.NewQueryMediator, fx.As(new(accountmonth.QueryMediatorer))),
		),
		fx.Provide(
			// Aggregate factory
			definitions.AsAggregateFactory(account.NewFactory),
			definitions.AsAggregateFactory(tagcategory.NewFactory),
			definitions.AsAggregateFactory(movementtype.NewFactory),
			definitions.AsAggregateFactory(accountmonth.NewFactory),
		),
		fx.Provide(
			// Event data registers
			definitions.AsEventDataRegister(account.NewEventRegister),
			definitions.AsEventDataRegister(tagcategory.NewEventRegister),
			definitions.AsEventDataRegister(movementtype.NewEventRegister),
			definitions.AsEventDataRegister(accountmonth.NewEventRegister),
		),
		fx.Provide(
			// Event Store DB
			fx.Annotate(eventstoredb.NewClient, fx.As(new(eventstoredb.EventStorerer))),
			fx.Annotate(eventstoredb.NewEventStoreFactory, fx.As(new(eventstoredb.EventStoreCreator))),
			fx.Annotate(eventstoredb.NewIdCreator, fx.As(new(eventstoredb.IdGenerator))),
		),
		fx.Provide(
			// Projections
			fx.Annotate(
				eventhandler.NewProjectionRegistry,
				fx.ParamTags(`group:"projectionProviders"`),
			),
			definitions.AsProjectionProvider(account.NewProjectionConfig),
			fx.Annotate(account.NewProjection, fx.As(new(account.ReadModelProjection))),
			definitions.AsProjectionProvider(tagcategory.NewProjectionConfig),
			fx.Annotate(tagcategory.NewProjection, fx.As(new(tagcategory.ReadModelProjection))),
			definitions.AsProjectionProvider(movementtype.NewProjectionConfig),
			fx.Annotate(movementtype.NewProjection, fx.As(new(movementtype.ReadModelProjection))),
			definitions.AsProjectionProvider(accountmonth.NewProjectionConfig),
			fx.Annotate(accountmonth.NewProjection, fx.As(new(accountmonth.ReadModelProjection))),
		),
		fx.Provide(
			// Sagas
			fx.Annotate(
				eventhandler.NewSagaRegistry,
				fx.ParamTags(`group:"sagaProviders"`),
			),
			definitions.AsSagaProvider(saga.NewAccountRegisterSaga),
			definitions.AsSagaProvider(saga.NewAccountMonthEndedSaga),
		),
		fx.Provide(
			// Read model repositories
			fx.Annotate(account.NewReadModelRepository, fx.As(new(account.ReadModeler))),
			fx.Annotate(tagcategory.NewReadModelRepository, fx.As(new(tagcategory.ReadModeler))),
			fx.Annotate(movementtype.NewReadModelRepository, fx.As(new(movementtype.ReadModeler))),
			fx.Annotate(accountmonth.NewReadModelRepository, fx.As(new(accountmonth.ReadModeler))),
		),
		fx.Provide(
			// Event horizon stuff
			fx.Annotate(bus.NewCommandHandler, fx.As(new(eventhorizon.CommandHandler))),

			// Mongo DB
			mongodb.NewMongoClient,
		),

		fx.Provide(
			// Websocket
			websocket.NewWebsocketUpgrader,
			fx.Annotate(
				websocket.NewModelUpdater,
				fx.As(new(definitions.Route)),
				fx.ResultTags(`group:"routes"`),
			),
		),

		// Start api server
		fx.Invoke(func(engine *gin.Engine) { /* Nothing to do here */ }),

		// Command handler
		fx.Invoke(account.RegisterCommandHandler),
		fx.Invoke(tagcategory.RegisterCommandHandler),
		fx.Invoke(movementtype.RegisterCommandHandler),
		fx.Invoke(accountmonth.RegisterCommandHandler),

		// Projection event stream subscriptions
		fx.Invoke(account.ProjectionSubscribeEventStream),
		fx.Invoke(tagcategory.ProjectionSubscribeEventStream),
		fx.Invoke(movementtype.ProjectionSubscribeEventStream),
		fx.Invoke(accountmonth.ProjectionSubscribeEventStream),

		// Saga event stream subscriptions
		fx.Invoke(saga.AccountRegisterSagaSubscribeEventStream),
		fx.Invoke(saga.AccountMonthEndedSagaSubscribeEventStream),

		// Event registration
		fx.Invoke(fx.Annotate(eventstoredb.RegisterEvents, fx.ParamTags(`group:"eventDataRegisters"`))),

		//fx.WithLogger(
		//	func(log *zap.Logger) fxevent.Logger {
		//		return &fxevent.ZapLogger{Logger: log}
		//	},
		//),
	).Run()
}

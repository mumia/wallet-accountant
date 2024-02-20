package main

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/commandhandler/bus"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"walletaccountant/account"
	"walletaccountant/accountcommand"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthcommand"
	"walletaccountant/accountmonthprojection"
	"walletaccountant/accountmonthquery"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountmonthsaga"
	"walletaccountant/accountprojection"
	"walletaccountant/accountquery"
	"walletaccountant/accountreadmodel"
	"walletaccountant/accountsaga"
	"walletaccountant/api"
	"walletaccountant/clock"
	"walletaccountant/common/saga"
	"walletaccountant/definitions"
	"walletaccountant/eventhandler"
	"walletaccountant/eventstoredb"
	"walletaccountant/mongodb"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypecommand"
	"walletaccountant/movementtypeprojection"
	"walletaccountant/movementtypequery"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategorycommand"
	"walletaccountant/tagcategoryprojection"
	"walletaccountant/tagcategoryquery"
	"walletaccountant/tagcategoryreadmodel"
	"walletaccountant/websocket"
)

func main() {
	fx.New(
		fx.Provide(
			fx.Annotate(api.NewServer, fx.ParamTags(`group:"routes"`, `group:"aggregateFactories"`)),
			definitions.AsRoute(api.NewHealthcheckApi),
			zap.NewDevelopment,
			clock.NewClock,
		),
		fx.Provide(
			// Command routes
			definitions.AsRoute(accountcommand.NewRegisterNewAccountApi),
			definitions.AsRoute(accountmonthcommand.NewAccountMonthRegisterNewMovementApi),
			definitions.AsRoute(accountmonthcommand.NewEndAccountMonthApi),
			definitions.AsRoute(movementtypecommand.NewRegisterNewMovementTypeApi),
			definitions.AsRoute(tagcategorycommand.NewNewTagAndCategoryApi),
			definitions.AsRoute(tagcategorycommand.NewNewTagWithExistingCategoryApi),
		),
		fx.Provide(
			// Query routes
			definitions.AsRoute(accountquery.NewReadAllAccountsApi),
			definitions.AsRoute(accountquery.NewReadAccountsApi),
			definitions.AsRoute(accountmonthquery.NewReadCurrentAccountMonthApi),
			definitions.AsRoute(movementtypequery.NewReadAllMovementTypesApi),
			definitions.AsRoute(movementtypequery.NewReadMovementTypeApi),
			definitions.AsRoute(movementtypequery.NewReadMovementTypeByAccountApi),
			definitions.AsRoute(tagcategoryquery.NewReadAllTagsApi),
		),
		fx.Provide(
			// CommandMediator
			fx.Annotate(accountcommand.NewCommandMediator, fx.As(new(accountcommand.CommandMediatorer))),
			fx.Annotate(tagcategorycommand.NewCommandMediator, fx.As(new(tagcategorycommand.CommandMediatorer))),
			fx.Annotate(movementtypecommand.NewCommandMediator, fx.As(new(movementtypecommand.CommandMediatorer))),
			fx.Annotate(accountmonthcommand.NewCommandMediator, fx.As(new(accountmonthcommand.CommandMediatorer))),
		),
		fx.Provide(
			// QueryMediator
			fx.Annotate(accountquery.NewQueryMediator, fx.As(new(accountquery.QueryMediatorer))),
			fx.Annotate(accountmonthquery.NewQueryMediator, fx.As(new(accountmonthquery.QueryMediatorer))),
			fx.Annotate(movementtypequery.NewQueryMediator, fx.As(new(movementtypequery.QueryMediatorer))),
			fx.Annotate(tagcategoryquery.NewQueryMediator, fx.As(new(tagcategoryquery.QueryMediatorer))),
		),
		fx.Provide(
			// Aggregate factory
			definitions.AsAggregateFactory(account.NewFactory),
			definitions.AsAggregateFactory(accountmonth.NewFactory),
			definitions.AsAggregateFactory(movementtype.NewFactory),
			definitions.AsAggregateFactory(tagcategory.NewFactory),
		),
		fx.Provide(
			// Event data registers
			definitions.AsEventDataRegister(account.NewEventRegister),
			definitions.AsEventDataRegister(accountmonth.NewEventRegister),
			definitions.AsEventDataRegister(movementtype.NewEventRegister),
			definitions.AsEventDataRegister(tagcategory.NewEventRegister),
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
			definitions.AsProjectionProvider(accountprojection.NewProjectionConfig),
			definitions.AsProjectionProvider(accountmonthprojection.NewProjectionConfig),
			definitions.AsProjectionProvider(movementtypeprojection.NewProjectionConfig),
			definitions.AsProjectionProvider(tagcategoryprojection.NewProjectionConfig),
			fx.Annotate(accountprojection.NewProjection, fx.As(new(accountprojection.ReadModelProjection))),
			fx.Annotate(accountmonthprojection.NewProjection, fx.As(new(accountmonthprojection.ReadModelProjection))),
			fx.Annotate(movementtypeprojection.NewProjection, fx.As(new(movementtypeprojection.ReadModelProjection))),
			fx.Annotate(tagcategoryprojection.NewProjection, fx.As(new(tagcategoryprojection.ReadModelProjection))),
		),
		fx.Provide(
			// Sagas
			fx.Annotate(
				eventhandler.NewSagaRegistry,
				fx.ParamTags(`group:"sagaProviders"`),
			),
			definitions.AsSagaProvider(accountsaga.NewAccountRegisterSaga),
			definitions.AsSagaProvider(accountmonthsaga.NewAccountMonthEndedSaga),
		),
		fx.Provide(
			// Read model repositories
			fx.Annotate(accountreadmodel.NewReadModelRepository, fx.As(new(accountreadmodel.ReadModeler))),
			fx.Annotate(accountmonthreadmodel.NewReadModelRepository, fx.As(new(accountmonthreadmodel.ReadModeler))),
			fx.Annotate(movementtypereadmodel.NewReadModelRepository, fx.As(new(movementtypereadmodel.ReadModeler))),
			fx.Annotate(tagcategoryreadmodel.NewReadModelRepository, fx.As(new(tagcategoryreadmodel.ReadModeler))),
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
		fx.Invoke(accountmonth.RegisterCommandHandler),
		fx.Invoke(movementtype.RegisterCommandHandler),
		fx.Invoke(tagcategory.RegisterCommandHandler),

		// Projection event stream subscriptions
		fx.Invoke(accountprojection.ProjectionSubscribeEventStream),
		fx.Invoke(accountmonthprojection.ProjectionSubscribeEventStream),
		fx.Invoke(movementtypeprojection.ProjectionSubscribeEventStream),
		fx.Invoke(tagcategoryprojection.ProjectionSubscribeEventStream),

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

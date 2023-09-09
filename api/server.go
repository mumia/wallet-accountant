package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"walletaccountant/definitions"
)

func NewServer(
	routes []Route,
	aggregateRegisters []definitions.AggregateFactory,
	lifecycle fx.Lifecycle,
) *gin.Engine {
	for _, aggregateRegister := range aggregateRegisters {
		eventhorizon.RegisterAggregate(aggregateRegister.Factory())
	}

	router := gin.Default()
	for _, route := range routes {
		method, pattern := route.Configuration()
		router.Handle(method, pattern, route.Handle)
	}

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					err := router.Run()
					if err != nil {
						fx.Error(err)
					}
				}()

				return nil
			},
		},
	)

	return router
}

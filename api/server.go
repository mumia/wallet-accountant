package api

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"time"
	"walletaccountant/definitions"
)

const frontendUrlName = "FRONTEND_URL"

func NewServer(
	routes []definitions.Route,
	aggregateRegisters []definitions.AggregateFactory,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) *gin.Engine {
	for _, aggregateRegister := range aggregateRegisters {
		eventhorizon.RegisterAggregate(aggregateRegister.Factory())
	}

	router := gin.Default()

	addCorsConfig(router)
	addRouteDefinitions(router, routes, logger)

	server := &http.Server{Addr: resolveAddress(logger), Handler: router}

	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				listener, err := net.Listen("tcp", server.Addr)
				if err != nil {
					return err
				}

				go func() {
					err := server.Serve(listener)
					if err != nil {
						fx.Error(err)
					}
				}()

				return nil
			},
			OnStop: func(ctx context.Context) error {
				err := server.Shutdown(ctx)
				if err != nil {
					return err
				}

				return nil
			},
		},
	)

	return router
}

func resolveAddress(logger *zap.Logger) string {
	port := os.Getenv("PORT")
	if port != "" {
		logger.Debug(fmt.Sprintf("Environment variable PORT=\"%s\"", port))

		return ":" + port
	}

	logger.Debug("Environment variable PORT is undefined. Using port :8080 by default")

	return ":8080"

}

func addCorsConfig(router *gin.Engine) {
	//config := cors.DefaultConfig()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{os.Getenv(frontendUrlName)},
		AllowMethods:     []string{"POST", "GET", "PUT", "PATCH"},
		AllowHeaders:     []string{"Origin"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		//AllowOriginFunc: func(origin string) bool {
		//	return origin == "https://github.com"
		//},
		MaxAge: 12 * time.Hour,
	}))
}

func addRouteDefinitions(router *gin.Engine, routes []definitions.Route, logger *zap.Logger) {
	for _, route := range routes {
		method, pattern := route.Configuration()
		//TODO new version of Golang will not require this forced copy
		handle := route.Handle
		router.Handle(method, pattern, handle)

		logger.Debug("new route added", zap.String("method", method), zap.String("pattern", pattern))
	}
}

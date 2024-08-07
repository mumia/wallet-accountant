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
	"strings"
	"time"
	"walletaccountant/definitions"
)

const frontendUrlsName = "FRONTEND_URLS"

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

	// Set a lower memory limit for multipart forms (default is 32 MiB)
	router.MaxMultipartMemory = 8 << 20 // 8 MiB

	router.Use(firstLogger(logger))

	addCorsConfig(router, logger)
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

func addCorsConfig(router *gin.Engine, logger *zap.Logger) {
	//config := cors.DefaultConfig()

	hosts := strings.Split(os.Getenv(frontendUrlsName), "|")

	logger.Debug("CORS configured allowed hosts", zap.Strings("hosts", hosts))

	router.Use(
		cors.New(
			cors.Config{
				AllowWebSockets:  true,
				AllowOrigins:     hosts,
				AllowMethods:     []string{"POST", "GET", "PUT", "PATCH"},
				AllowHeaders:     []string{"Origin", "Content-Action, Content-Type"},
				ExposeHeaders:    []string{"Content-Length", "Content-Action"},
				AllowCredentials: true,
				//AllowOriginFunc: func(origin string) bool {
				//	return origin == "https://github.com"
				//},
				MaxAge: 12 * time.Hour,
			},
		),
	)
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

func firstLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		logger.Debug(
			"Incoming call",
			zap.String("remoteAddr", ctx.Request.RemoteAddr),
			zap.String("host", ctx.Request.Host),
			zap.String("requestUri", ctx.Request.RequestURI),
			zap.String("method", ctx.Request.Method),
		)

		ctx.Next()
	}
}

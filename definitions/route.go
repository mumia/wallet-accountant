package definitions

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

type Route interface {
	Configuration() (string, string)
	Handle(*gin.Context)
}

func AsRoute(f any) any {
	return fx.Annotate(
		f,
		fx.As(new(Route)),
		fx.ResultTags(`group:"routes"`),
	)
}

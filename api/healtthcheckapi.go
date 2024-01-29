package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"walletaccountant/definitions"
)

var _ definitions.Route = &HealthcheckApi{}

type HealthcheckApi struct {
}

func NewHealthcheckApi() *HealthcheckApi {
	return &HealthcheckApi{}
}

func (api *HealthcheckApi) Configuration() (string, string) {
	return http.MethodGet, "/healthcheck"
}

func (api *HealthcheckApi) Handle(ctx *gin.Context) {
	ctx.Status(http.StatusOK)
}

package account

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/api"
)

var _ api.Route = &RegisterNewAccountApi{}

type RegisterNewAccountApi struct {
	mediator *Mediator
	log      *zap.Logger
}

func NewRegisterApi(mediator *Mediator, log *zap.Logger) *RegisterNewAccountApi {
	return &RegisterNewAccountApi{mediator: mediator, log: log}
}

func (apiCreate *RegisterNewAccountApi) Configuration() (string, string) {
	return http.MethodPost, "/account"
}

func (apiCreate *RegisterNewAccountApi) Handle(ctx *gin.Context) {
	var transferObject RegisterNewAccountTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		apiCreate.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	newAccountId, err := apiCreate.mediator.RegisterNewAccount(ctx, transferObject)
	if err != nil {
		apiCreate.log.Error("Failed to register new account", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"accountId": newAccountId})
}

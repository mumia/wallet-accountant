package commandapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/account"
	"walletaccountant/definitions"
)

var _ definitions.Route = &RegisterNewAccountApi{}

type RegisterNewAccountApi struct {
	mediator account.CommandMediatorer
	log      *zap.Logger
}

func NewRegisterNewAccountApi(mediator account.CommandMediatorer, log *zap.Logger) *RegisterNewAccountApi {
	return &RegisterNewAccountApi{mediator: mediator, log: log}
}

func (api *RegisterNewAccountApi) Configuration() (string, string) {
	return http.MethodPost, "/account"
}

func (api *RegisterNewAccountApi) Handle(ctx *gin.Context) {
	var transferObject account.RegisterNewAccountTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		err := account.GenericError(err, nil)

		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	newAccountId, err := api.mediator.RegisterNewAccount(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to register new account", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"accountId": newAccountId})
}

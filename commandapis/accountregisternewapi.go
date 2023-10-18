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

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	newAccountId, err := api.mediator.RegisterNewAccount(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to register new account", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"accountId": newAccountId})
}

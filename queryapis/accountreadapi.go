package queryapis

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/account"
	"walletaccountant/definitions"
)

var _ definitions.Route = &ReadAccountsApi{}

type request struct {
	AccountId string `uri:"accountId"  binding:"required,uuid"`
}

type ReadAccountsApi struct {
	mediator account.QueryMediatorer
	log      *zap.Logger
}

func NewReadAccountsApi(mediator account.QueryMediatorer, log *zap.Logger) *ReadAccountsApi {
	return &ReadAccountsApi{mediator: mediator, log: log}
}

func (api *ReadAccountsApi) Configuration() (string, string) {
	return http.MethodGet, "/account/:accountId"
}

func (api *ReadAccountsApi) Handle(ctx *gin.Context) {
	var request request

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}

	accountId := uuid.MustParse(request.AccountId)
	accountResult, err := api.mediator.Account(ctx, &accountId)
	if err != nil {
		api.log.Error("Failed to get account", zap.Error(err), zap.Any("request", request))

		status := http.StatusInternalServerError
		if _, ok := err.(account.ErrorAccountEntityNotFound); ok {
			status = http.StatusNotFound
		}

		ctx.JSON(status, gin.H{"error": err.Error()})

		return
	}

	ctx.JSON(http.StatusOK, accountResult)
}

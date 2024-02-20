package accountquery

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
)

var _ definitions.Route = &ReadAllAccountsApi{}

type ReadAllAccountsApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadAllAccountsApi(mediator QueryMediatorer, log *zap.Logger) *ReadAllAccountsApi {
	return &ReadAllAccountsApi{mediator: mediator, log: log}
}

func (api *ReadAllAccountsApi) Configuration() (string, string) {
	return http.MethodGet, "/accounts"
}

func (api *ReadAllAccountsApi) Handle(ctx *gin.Context) {
	accounts, err := api.mediator.Accounts(ctx)

	if err != nil {
		api.log.Error("Failed to query all accounts", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	if accounts == nil {
		accounts = make([]*accountreadmodel.Entity, 0)
	}

	ctx.AsciiJSON(http.StatusOK, accounts)
}

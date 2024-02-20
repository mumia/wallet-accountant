package accountmonthquery

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/accountmonth"
	"walletaccountant/definitions"
)

var _ definitions.Route = &ReadCurrentAccountMonthApi{}

type currentAccountMonthRequest struct {
	AccountId string `uri:"accountId"  binding:"required,uuid"`
}

type ReadCurrentAccountMonthApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadCurrentAccountMonthApi(mediator QueryMediatorer, log *zap.Logger) *ReadCurrentAccountMonthApi {
	return &ReadCurrentAccountMonthApi{mediator: mediator, log: log}
}

func (api *ReadCurrentAccountMonthApi) Configuration() (string, string) {
	return http.MethodGet, "/account-month/:accountId"
}

func (api *ReadCurrentAccountMonthApi) Handle(ctx *gin.Context) {
	var request currentAccountMonthRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	accountId := uuid.MustParse(request.AccountId)
	accountMonthResult, err := api.mediator.AccountMonth(ctx, &accountId)
	if err != nil {
		api.log.Error("Failed to get current account month", zap.Error(err), zap.Any("request", request))

		status := http.StatusInternalServerError
		if err.Code == accountmonth.NonExistentAccountIdErrorCode {
			status = http.StatusBadRequest
		}
		if err.Code == accountmonth.NonExistentAccountMonthErrorCode {
			status = http.StatusNotFound
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusOK, accountMonthResult)
}

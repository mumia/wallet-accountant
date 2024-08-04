package ledgercommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
)

var _ definitions.Route = &EndAccountMonthApi{}

type EndAccountMonthApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewEndAccountMonthApi(
	mediator CommandMediatorer,
	log *zap.Logger,
) *EndAccountMonthApi {
	return &EndAccountMonthApi{
		mediator: mediator,
		log:      log,
	}
}

func (api *EndAccountMonthApi) Configuration() (string, string) {
	return http.MethodPut, "/ledger"
}

func (api *EndAccountMonthApi) Handle(context *gin.Context) {
	var transferObject EndAccountMonthTransferObject

	if err := context.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		context.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	err := api.mediator.EndAccountMonth(context, transferObject)
	if err != nil {
		api.log.Error("Failed to end account month", zap.Error(err))

		responseCode := http.StatusInternalServerError
		switch err.Code {
		case ledger.NonExistentAccountErrorCode:
			responseCode = http.StatusBadRequest
		case ledger.MismatchedActiveMonthErrorCode:
			responseCode = http.StatusBadRequest
		case ledger.NonExistentAccountMonthErrorCode:
			responseCode = http.StatusBadRequest
		case ledger.AlreadyEndedErrorCode:
			responseCode = http.StatusBadRequest
		case ledger.MismatchedEndBalanceErrorCode:
			responseCode = http.StatusBadRequest
		}

		context.JSON(responseCode, err)

		return
	}

	context.JSON(http.StatusNoContent, gin.H{})
}

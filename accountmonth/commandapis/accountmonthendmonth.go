package commandapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/accountmonth"
	"walletaccountant/definitions"
)

var _ definitions.Route = &EndAccountMonthApi{}

type EndAccountMonthApi struct {
	mediator accountmonth.CommandMediatorer
	log      *zap.Logger
}

func NewEndAccountMonthApi(
	mediator accountmonth.CommandMediatorer,
	log *zap.Logger,
) *EndAccountMonthApi {
	return &EndAccountMonthApi{
		mediator: mediator,
		log:      log,
	}
}

func (api *EndAccountMonthApi) Configuration() (string, string) {
	return http.MethodPut, "/account-month"
}

func (api *EndAccountMonthApi) Handle(context *gin.Context) {
	var transferObject accountmonth.EndAccountMonthTransferObject

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
		case accountmonth.NonExistentAccountErrorCode:
			responseCode = http.StatusBadRequest
		case accountmonth.MismatchedActiveMonthErrorCode:
			responseCode = http.StatusBadRequest
		case accountmonth.NonExistentAccountMonthErrorCode:
			responseCode = http.StatusBadRequest
		case accountmonth.AlreadyEndedErrorCode:
			responseCode = http.StatusBadRequest
		case accountmonth.MismatchedEndBalanceErrorCode:
			responseCode = http.StatusBadRequest
		}

		context.JSON(responseCode, err)

		return
	}

	context.JSON(http.StatusNoContent, gin.H{})
}

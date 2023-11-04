package commandapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/accountmonth"
	"walletaccountant/definitions"
)

var _ definitions.Route = &RegisterNewAccountMovementApi{}

type RegisterNewAccountMovementApi struct {
	mediator accountmonth.CommandMediatorer
	log      *zap.Logger
}

func NewAccountMonthRegisterNewMovementApi(
	mediator accountmonth.CommandMediatorer,
	log *zap.Logger,
) *RegisterNewAccountMovementApi {
	return &RegisterNewAccountMovementApi{
		mediator: mediator,
		log:      log,
	}
}

func (api *RegisterNewAccountMovementApi) Configuration() (string, string) {
	return http.MethodPost, "/account-month/account-movement"
}

func (api *RegisterNewAccountMovementApi) Handle(context *gin.Context) {
	var transferObject accountmonth.RegisterNewAccountMovementTransferObject

	if err := context.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		context.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	err := api.mediator.RegisterNewAccountMovement(context, transferObject)
	if err != nil {
		api.log.Error("Failed to register new account movement", zap.Error(err))

		responseCode := http.StatusInternalServerError
		switch err.Code {
		case accountmonth.NonExistentAccountErrorCode:
			responseCode = http.StatusBadRequest

		case accountmonth.MismatchedActiveMonthErrorCode:
			responseCode = http.StatusBadRequest

		case accountmonth.NonExistentMovementTypeErrorCode:
			responseCode = http.StatusBadRequest

		case accountmonth.MismatchedAccountIdErrorCode:
			responseCode = http.StatusBadRequest

		}

		context.JSON(responseCode, err)

		return
	}

	context.JSON(http.StatusNoContent, nil)
}

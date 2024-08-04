package ledgercommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
)

var _ definitions.Route = &RegisterNewAccountMovementApi{}

type RegisterNewAccountMovementApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewAccountMonthRegisterNewMovementApi(
	mediator CommandMediatorer,
	log *zap.Logger,
) *RegisterNewAccountMovementApi {
	return &RegisterNewAccountMovementApi{
		mediator: mediator,
		log:      log,
	}
}

func (api *RegisterNewAccountMovementApi) Configuration() (string, string) {
	return http.MethodPost, "/ledger/account-movement"
}

func (api *RegisterNewAccountMovementApi) Handle(context *gin.Context) {
	var transferObject RegisterNewAccountMovementTransferObject

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
		case ledger.NonExistentAccountErrorCode:
			responseCode = http.StatusBadRequest

		case ledger.MismatchedActiveMonthErrorCode:
			responseCode = http.StatusBadRequest

		case ledger.NonExistentMovementTypeErrorCode:
			responseCode = http.StatusBadRequest

		case ledger.MismatchedAccountIdErrorCode:
			responseCode = http.StatusBadRequest

		}

		context.JSON(responseCode, err)

		return
	}

	context.Status(http.StatusCreated)
}

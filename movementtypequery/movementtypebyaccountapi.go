package movementtypequery

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
)

var _ definitions.Route = &ReadMovementTypeApi{}

type movementTypeByAccountRequest struct {
	AccountId string `uri:"accountId"  binding:"required,uuid"`
}

type ReadMovementTypeByAccountApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadMovementTypeByAccountApi(
	mediator QueryMediatorer,
	log *zap.Logger,
) *ReadMovementTypeByAccountApi {
	return &ReadMovementTypeByAccountApi{mediator: mediator, log: log}
}

func (api *ReadMovementTypeByAccountApi) Configuration() (string, string) {
	return http.MethodGet, "/movement-type/account/:accountId"
}

func (api *ReadMovementTypeByAccountApi) Handle(ctx *gin.Context) {
	var request movementTypeByAccountRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	accountId := uuid.MustParse(request.AccountId)
	movementTypeResult, err := api.mediator.MovementTypesByAccountId(ctx, &accountId)
	if err != nil {
		api.log.Error(
			"Failed to get movement types by account id",
			zap.Error(err),
			zap.Any("request", request),
		)

		status := http.StatusInternalServerError
		if err.Code == movementtype.NonexistentMovementTypeErrorCode {
			status = http.StatusNotFound
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusOK, movementTypeResult)
}

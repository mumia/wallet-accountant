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

type movementTypeRequest struct {
	MovementTypeId string `uri:"movementTypeId"  binding:"required,uuid"`
}

type ReadMovementTypeApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadMovementTypeApi(mediator QueryMediatorer, log *zap.Logger) *ReadMovementTypeApi {
	return &ReadMovementTypeApi{mediator: mediator, log: log}
}

func (api *ReadMovementTypeApi) Configuration() (string, string) {
	return http.MethodGet, "/movement-type/:movementTypeId"
}

func (api *ReadMovementTypeApi) Handle(ctx *gin.Context) {
	var request movementTypeRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	movementTypeId := uuid.MustParse(request.MovementTypeId)
	movementTypeResult, err := api.mediator.MovementType(ctx, &movementTypeId)
	if err != nil {
		api.log.Error("Failed to get movement type", zap.Error(err), zap.Any("request", request))

		status := http.StatusInternalServerError
		if err.Code == movementtype.NonexistentMovementTypeErrorCode {
			status = http.StatusNotFound
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusOK, movementTypeResult)
}

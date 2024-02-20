package movementtypecommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
)

var _ definitions.Route = &RegisterNewMovementTypeApi{}

type RegisterNewMovementTypeApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewRegisterNewMovementTypeApi(mediator CommandMediatorer, log *zap.Logger) *RegisterNewMovementTypeApi {
	return &RegisterNewMovementTypeApi{mediator: mediator, log: log}
}

func (api *RegisterNewMovementTypeApi) Configuration() (string, string) {
	return http.MethodPost, "/movement-type"
}

func (api *RegisterNewMovementTypeApi) Handle(ctx *gin.Context) {
	var transferObject RegisterNewMovementTypeTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	newMovementTypeId, err := api.mediator.RegisterNewMovementType(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to register new movement type", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"movementTypeId": newMovementTypeId})
}

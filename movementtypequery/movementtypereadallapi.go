package movementtypequery

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/movementtypereadmodel"
)

var _ definitions.Route = &ReadAllMovementTypeApi{}

type ReadAllMovementTypeApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadAllMovementTypesApi(mediator QueryMediatorer, log *zap.Logger) *ReadAllMovementTypeApi {
	return &ReadAllMovementTypeApi{mediator: mediator, log: log}
}

func (api *ReadAllMovementTypeApi) Configuration() (string, string) {
	return http.MethodGet, "/movement-type"
}

func (api *ReadAllMovementTypeApi) Handle(ctx *gin.Context) {
	movementTypes, err := api.mediator.MovementTypes(ctx)

	if err != nil {
		api.log.Error("Failed to query all tags", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	if movementTypes == nil {
		movementTypes = make([]*movementtypereadmodel.Entity, 0)
	}

	ctx.AsciiJSON(http.StatusOK, movementTypes)
}

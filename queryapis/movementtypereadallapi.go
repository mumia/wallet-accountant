package queryapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/movementtype"
)

var _ definitions.Route = &ReadAllMovementTypeApi{}

type ReadAllMovementTypeApi struct {
	mediator movementtype.QueryMediatorer
	log      *zap.Logger
}

func NewReadAllMovementTypeApi(mediator movementtype.QueryMediatorer, log *zap.Logger) *ReadAllMovementTypeApi {
	return &ReadAllMovementTypeApi{mediator: mediator, log: log}
}

func (api *ReadAllMovementTypeApi) Configuration() (string, string) {
	return http.MethodGet, "/movement-types"
}

func (api *ReadAllMovementTypeApi) Handle(ctx *gin.Context) {
	accounts, err := api.mediator.MovementTypes(ctx)

	if err != nil {
		api.log.Error("Failed to query all tags", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.AsciiJSON(http.StatusOK, accounts)
}

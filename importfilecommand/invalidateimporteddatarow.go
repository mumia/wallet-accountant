package importfilecommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
)

var _ definitions.Route = &InvalidateImportedDataRowApi{}

type InvalidateImportedDataRowApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewInvalidateImportedDataRowApi(mediator CommandMediatorer, log *zap.Logger) *InvalidateImportedDataRowApi {
	return &InvalidateImportedDataRowApi{mediator: mediator, log: log}
}

func (api *InvalidateImportedDataRowApi) Configuration() (string, string) {
	return http.MethodPost, "/import-file/data-row/invalidate"
}

func (api *InvalidateImportedDataRowApi) Handle(context *gin.Context) {
	var transferObject InvalidateFileDataRowTransferObject

	if err := context.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		context.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	err := api.mediator.InvalidateFileDataRow(context, transferObject)
	if err != nil {
		api.log.Error("Failed to invalidate imported movement", zap.Error(err))

		responseCode := http.StatusInternalServerError

		context.JSON(responseCode, err)

		return
	}

	context.Status(http.StatusNoContent)
}

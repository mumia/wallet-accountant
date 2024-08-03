package importfilecommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
)

var _ definitions.Route = &VerifyImportedDataRowApi{}

type VerifyImportedDataRowApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewVerifyImportedDataRowApi(mediator CommandMediatorer, log *zap.Logger) *VerifyImportedDataRowApi {
	return &VerifyImportedDataRowApi{mediator: mediator, log: log}
}

func (api *VerifyImportedDataRowApi) Configuration() (string, string) {
	return http.MethodPost, "/import-file/data-row/verify"
}

func (api *VerifyImportedDataRowApi) Handle(context *gin.Context) {
	var transferObject VerifyFileDataRowTransferObject

	if err := context.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		context.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	err := api.mediator.VerifyFileDataRow(context, transferObject)
	if err != nil {
		api.log.Error("Failed to verify imported movement", zap.Error(err))

		responseCode := http.StatusInternalServerError

		context.JSON(responseCode, err)

		return
	}

	context.Status(http.StatusNoContent)
}

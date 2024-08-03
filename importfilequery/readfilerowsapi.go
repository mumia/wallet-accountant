package importfilequery

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/importfile"
)

var _ definitions.Route = &ReadImportFileRowsApi{}

type ReadImportFileRowsApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadImportFileRowsApi(mediator QueryMediatorer, log *zap.Logger) *ReadImportFileRowsApi {
	return &ReadImportFileRowsApi{mediator: mediator, log: log}
}

func (api *ReadImportFileRowsApi) Configuration() (string, string) {
	return http.MethodGet, "/import-file/:importFileId/rows"
}

func (api *ReadImportFileRowsApi) Handle(ctx *gin.Context) {
	var request importFileRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	importFileId := uuid.MustParse(request.ImportFileId)
	importFile, err := api.mediator.ImportFileRows(ctx, &importFileId)
	if err != nil {
		api.log.Error("Failed to get import file rows", zap.Error(err), zap.Any("request", request))

		status := http.StatusInternalServerError
		if err.Code == importfile.NonExistentImportFileCode {
			status = http.StatusNotFound
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusOK, importFile)
}

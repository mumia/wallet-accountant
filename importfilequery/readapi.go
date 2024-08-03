package importfilequery

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/importfile"
)

var _ definitions.Route = &ReadImportFileApi{}

type ReadImportFileApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadImportFileApi(mediator QueryMediatorer, log *zap.Logger) *ReadImportFileApi {
	return &ReadImportFileApi{mediator: mediator, log: log}
}

func (api *ReadImportFileApi) Configuration() (string, string) {
	return http.MethodGet, "/import-file/:importFileId"
}

func (api *ReadImportFileApi) Handle(ctx *gin.Context) {
	var request importFileRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	importFileId := uuid.MustParse(request.ImportFileId)
	importFile, err := api.mediator.ImportFile(ctx, &importFileId)
	if err != nil {
		api.log.Error("Failed to get import file", zap.Error(err), zap.Any("request", request))

		status := http.StatusInternalServerError
		if err.Code == importfile.NonExistentImportFileCode {
			status = http.StatusNotFound
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusOK, importFile)
}

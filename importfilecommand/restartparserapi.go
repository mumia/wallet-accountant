package importfilecommand

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
)

var _ definitions.Route = &RestartImportFileParserApi{}

type importFileRequest struct {
	ImportFileId string `uri:"importFileId"  binding:"required,uuid"`
}

type RestartImportFileParserApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewRestartImportFileParserApi(mediator CommandMediatorer, log *zap.Logger) *RestartImportFileParserApi {
	log.With(zap.String("struct", "RestartImportFileParserApi"))

	return &RestartImportFileParserApi{mediator: mediator, log: log}
}

func (api *RestartImportFileParserApi) Configuration() (string, string) {
	return http.MethodGet, "/import-file/:importFileId/restart"
}

func (api *RestartImportFileParserApi) Handle(ctx *gin.Context) {
	var request importFileRequest

	if err := ctx.ShouldBindUri(&request); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	importFileId := uuid.MustParse(request.ImportFileId)
	err := api.mediator.RestartFileParse(ctx, &importFileId)
	if err != nil {
		api.log.Error("Failed to restart import file parse", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.Status(http.StatusNoContent)
}

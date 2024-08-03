package importfilequery

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/importfilereadmodel"
)

var _ definitions.Route = &ReadAllImportFilesApi{}

type ReadAllImportFilesApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadAllImportFilesApi(mediator QueryMediatorer, log *zap.Logger) *ReadAllImportFilesApi {
	return &ReadAllImportFilesApi{mediator: mediator, log: log}
}

func (api *ReadAllImportFilesApi) Configuration() (string, string) {
	return http.MethodGet, "/import-files"
}

func (api *ReadAllImportFilesApi) Handle(ctx *gin.Context) {
	importFiles, err := api.mediator.ImportFiles(ctx)

	if err != nil {
		api.log.Error("Failed to query all import files", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	if importFiles == nil {
		importFiles = make([]*importfilereadmodel.Entity, 0)
	}

	ctx.AsciiJSON(http.StatusOK, importFiles)
}

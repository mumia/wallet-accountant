package importfilecommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"os"
	"path/filepath"
	"walletaccountant/definitions"
)

var _ definitions.Route = &RegisterNewImportFileApi{}

type RegisterNewImportFileApi struct {
	mediator       CommandMediatorer
	fileUploadPath string
	log            *zap.Logger
}

func NewRegisterNewImportFileApi(mediator CommandMediatorer, log *zap.Logger) *RegisterNewImportFileApi {
	log.With(zap.String("struct", "RegisterNewImportFileApi"))

	fileUploadPath := os.Getenv("FILE_UPLOAD_PATH")

	return &RegisterNewImportFileApi{mediator: mediator, fileUploadPath: fileUploadPath, log: log}
}

func (api *RegisterNewImportFileApi) Configuration() (string, string) {
	return http.MethodPost, "/import-file"
}

func (api *RegisterNewImportFileApi) Handle(ctx *gin.Context) {
	var transferObject RegisterNewImportFileFormTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	// Save uploaded file
	file := transferObject.File
	filename := filepath.Base(file.Filename)
	fullUploadPath := filepath.Join(api.fileUploadPath, filename)

	if err := ctx.SaveUploadedFile(file, fullUploadPath); err != nil {
		api.log.Error("Failed to save uploaded file", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	newImportFileId, err := api.mediator.RegisterNewImportFile(
		ctx,
		RegisterNewImportFileTransferObject{
			AccountId: transferObject.AccountId,
			Filename:  filename,
		},
	)
	if err != nil {
		api.log.Error("Failed to register new import file", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"importFileId": newImportFileId})
}

package tagcategorycommand

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ definitions.Route = &NewTagWithExistingCategoryApi{}

type NewTagWithExistingCategoryApi struct {
	mediator CommandMediatorer
	log      *zap.Logger
}

func NewNewTagWithExistingCategoryApi(
	mediator CommandMediatorer,
	log *zap.Logger,
) *NewTagWithExistingCategoryApi {
	return &NewTagWithExistingCategoryApi{mediator: mediator, log: log}
}

func (api *NewTagWithExistingCategoryApi) Configuration() (string, string) {
	return http.MethodPost, "/tag"
}

func (api *NewTagWithExistingCategoryApi) Handle(ctx *gin.Context) {
	var transferObject AddNewTagToExistingCategoryTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	newTagId, err := api.mediator.AddNewTagToExistingCategory(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to add new tag to existing category", zap.Error(err))

		status := http.StatusInternalServerError
		if err.Code == tagcategory.NonexistentCategoryErrorCode || err.Code == tagcategory.NameAlreadyExistsCode {
			status = http.StatusBadRequest
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"tagId": newTagId})
}

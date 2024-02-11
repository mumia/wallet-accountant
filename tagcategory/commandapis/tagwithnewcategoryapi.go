package commandapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ definitions.Route = &NewTagAndCategoryApi{}

type NewTagAndCategoryApi struct {
	mediator tagcategory.CommandMediatorer
	log      *zap.Logger
}

func NewNewTagAndCategoryApi(mediator tagcategory.CommandMediatorer, log *zap.Logger) *NewTagAndCategoryApi {
	return &NewTagAndCategoryApi{mediator: mediator, log: log}
}

func (api *NewTagAndCategoryApi) Configuration() (string, string) {
	return http.MethodPost, "/tag-category"
}

func (api *NewTagAndCategoryApi) Handle(ctx *gin.Context) {
	var transferObject tagcategory.AddNewTagToNewCategoryTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	newTagId, newCategoryId, err := api.mediator.AddNewTagToNewCategory(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to add new tag and new category", zap.Error(err))

		status := http.StatusInternalServerError
		if err.Code == tagcategory.CategoryNameAlreadyExistsCode || err.Code == tagcategory.NameAlreadyExistsCode {
			status = http.StatusBadRequest
		}

		ctx.JSON(status, err)

		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"tagId": newTagId, "tagCategoryId": newCategoryId})
}

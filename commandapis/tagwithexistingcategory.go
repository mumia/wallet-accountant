package commandapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ definitions.Route = &NewTagApi{}

type NewTagApi struct {
	mediator tagcategory.CommandMediatorer
	log      *zap.Logger
}

func NewNewTagApi(mediator tagcategory.CommandMediatorer, log *zap.Logger) *NewTagApi {
	return &NewTagApi{mediator: mediator, log: log}
}

func (api *NewTagApi) Configuration() (string, string) {
	return http.MethodPost, "/tag"
}

func (api *NewTagApi) Handle(ctx *gin.Context) {
	var transferObject tagcategory.AddNewTagToExistingCategoryTransferObject

	if err := ctx.ShouldBind(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, account.GenericError(err, nil))

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

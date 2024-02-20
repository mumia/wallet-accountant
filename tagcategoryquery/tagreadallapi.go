package tagcategoryquery

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/tagcategorycommand"
	"walletaccountant/tagcategoryreadmodel"
)

var _ definitions.Route = &ReadAllTagsApi{}

type ReadAllTagsApi struct {
	mediator QueryMediatorer
	log      *zap.Logger
}

func NewReadAllTagsApi(mediator QueryMediatorer, log *zap.Logger) *ReadAllTagsApi {
	return &ReadAllTagsApi{mediator: mediator, log: log}
}

func (api *ReadAllTagsApi) Configuration() (string, string) {
	return http.MethodGet, "/tags"
}

func (api *ReadAllTagsApi) Handle(ctx *gin.Context) {
	var transferObject tagcategorycommand.FiltersTransferObject

	if err := ctx.ShouldBindQuery(&transferObject); err != nil {
		api.log.Error("Failed to bind request", zap.Error(err))

		ctx.JSON(http.StatusBadRequest, definitions.GenericError(err, nil))

		return
	}

	tags, err := api.mediator.Tags(ctx, transferObject)
	if err != nil {
		api.log.Error("Failed to query all tags", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	if tags == nil {
		tags = make([]*tagcategoryreadmodel.CategoryEntity, 0)
	}

	ctx.AsciiJSON(http.StatusOK, tags)
}

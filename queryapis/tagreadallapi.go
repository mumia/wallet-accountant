package queryapis

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
)

var _ definitions.Route = &ReadAllTagsApi{}

type ReadAllTagsApi struct {
	mediator tagcategory.QueryMediatorer
	log      *zap.Logger
}

func NewReadAllTagsApi(mediator tagcategory.QueryMediatorer, log *zap.Logger) *ReadAllTagsApi {
	return &ReadAllTagsApi{mediator: mediator, log: log}
}

func (api *ReadAllTagsApi) Configuration() (string, string) {
	return http.MethodGet, "/tags"
}

func (api *ReadAllTagsApi) Handle(ctx *gin.Context) {
	accounts, err := api.mediator.Tags(ctx)

	if err != nil {
		api.log.Error("Failed to query all tags", zap.Error(err))

		ctx.JSON(http.StatusInternalServerError, err)

		return
	}

	ctx.AsciiJSON(http.StatusOK, accounts)
}

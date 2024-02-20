package tagcategoryquery

import (
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"walletaccountant/definitions"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategorycommand"
	"walletaccountant/tagcategoryreadmodel"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	Tags(ctx *gin.Context, filters tagcategorycommand.FiltersTransferObject) ([]*tagcategoryreadmodel.CategoryEntity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository tagcategoryreadmodel.ReadModeler
}

func NewQueryMediator(repository tagcategoryreadmodel.ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) Tags(
	ctx *gin.Context,
	filters tagcategorycommand.FiltersTransferObject,
) ([]*tagcategoryreadmodel.CategoryEntity, *definitions.WalletAccountantError) {
	if len(filters.Filters) > 0 {
		return mediator.filterTags(ctx, filters)
	}

	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

func (mediator QueryMediator) filterTags(
	ctx *gin.Context,
	filters tagcategorycommand.FiltersTransferObject,
) ([]*tagcategoryreadmodel.CategoryEntity, *definitions.WalletAccountantError) {
	var tagIds []tagcategory.TagId
	for _, filter := range filters.Filters {
		tagIds = append(tagIds, uuid.MustParse(filter))
	}

	entities, err := mediator.repository.GetByTagIds(ctx, tagIds)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

package tagcategoryquery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
	"walletaccountant/tagcategorycommand"
	"walletaccountant/tagcategoryreadmodel"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	TagsFn func(ctx *gin.Context, filters tagcategorycommand.FiltersTransferObject) ([]*tagcategoryreadmodel.CategoryEntity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) Tags(ctx *gin.Context, filters tagcategorycommand.FiltersTransferObject) ([]*tagcategoryreadmodel.CategoryEntity, *definitions.WalletAccountantError) {
	if mock != nil && mock.TagsFn != nil {
		return mock.TagsFn(ctx, filters)
	}

	return nil, nil
}

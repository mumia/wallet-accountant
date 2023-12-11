package tagcategory

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	TagsFn func(ctx *gin.Context, filters FiltersTransferObject) ([]*CategoryEntity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) Tags(ctx *gin.Context, filters FiltersTransferObject) ([]*CategoryEntity, *definitions.WalletAccountantError) {
	if mock != nil && mock.TagsFn != nil {
		return mock.TagsFn(ctx, filters)
	}

	return nil, nil
}

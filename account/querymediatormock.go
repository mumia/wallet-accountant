package account

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	AccountFn  func(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError)
	AccountsFn func(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) Account(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountFn != nil {
		return mock.AccountFn(ctx, accountId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) Accounts(ctx *gin.Context) ([]*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountsFn != nil {
		return mock.AccountsFn(ctx)
	}

	return nil, nil
}

package accountquery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	AccountFn  func(ctx *gin.Context, accountId *account.Id) (*accountreadmodel.Entity, *definitions.WalletAccountantError)
	AccountsFn func(ctx *gin.Context) ([]*accountreadmodel.Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) Account(ctx *gin.Context, accountId *account.Id) (*accountreadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountFn != nil {
		return mock.AccountFn(ctx, accountId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) Accounts(ctx *gin.Context) ([]*accountreadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountsFn != nil {
		return mock.AccountsFn(ctx)
	}

	return nil, nil
}

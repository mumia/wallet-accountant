package ledgerquery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/ledger"
	"walletaccountant/ledgerreadmodel"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	AccountMonthFn func(ctx *gin.Context, accountId *ledger.Id) (*ledgerreadmodel.Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) AccountMonth(
	ctx *gin.Context,
	accountId *account.Id,
) (*ledgerreadmodel.Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountMonthFn != nil {
		return mock.AccountMonthFn(ctx, accountId)
	}

	return nil, nil
}

package accountmonth

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	AccountMonthFn func(ctx *gin.Context, accountId *Id) (*Entity, *definitions.WalletAccountantError)
}

func (mock *QueryMediatorMock) AccountMonth(
	ctx *gin.Context,
	accountId *account.Id,
) (*Entity, *definitions.WalletAccountantError) {
	if mock != nil && mock.AccountMonthFn != nil {
		return mock.AccountMonthFn(ctx, accountId)
	}

	return nil, nil
}

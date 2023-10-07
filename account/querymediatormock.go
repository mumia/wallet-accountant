package account

import "github.com/gin-gonic/gin"

var _ QueryMediatorer = &QueryMediatorMock{}

type QueryMediatorMock struct {
	AccountFn  func(ctx *gin.Context, accountId *Id) (*Entity, error)
	AccountsFn func(ctx *gin.Context) ([]*Entity, error)
}

func (mock *QueryMediatorMock) Account(ctx *gin.Context, accountId *Id) (*Entity, error) {
	if mock != nil && mock.AccountFn != nil {
		return mock.AccountFn(ctx, accountId)
	}

	return nil, nil
}

func (mock *QueryMediatorMock) Accounts(ctx *gin.Context) ([]*Entity, error) {
	if mock != nil && mock.AccountsFn != nil {
		return mock.AccountsFn(ctx)
	}

	return nil, nil
}

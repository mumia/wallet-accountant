package accountmonthquery_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/account"
	"walletaccountant/accountmonthquery"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountreadmodel"
)

func TestQueryMediator_AccountMonth(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	accountTimesCalled := 0
	accountRepositoryMock := accountreadmodel.ReadModelRepositoryMock{
		GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
			accountTimesCalled++

			return &accountEntity, nil
		},
	}

	timesCalled := 0
	repositoryMock := accountmonthreadmodel.ReadModelRepositoryMock{
		GetByAccountActiveMonthFn: func(ctx context.Context, account *accountreadmodel.Entity) (*accountmonthreadmodel.Entity, error) {
			timesCalled++

			return &accountMonthEntity, nil
		},
	}

	queryMediator := accountmonthquery.NewQueryMediator(&repositoryMock, &accountRepositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.AccountMonth(&ctx, &accountId1)
	requires.Nil(err)

	asserts.Equal(&accountMonthEntity, actualAccount)

	asserts.Equal(1, timesCalled)
}

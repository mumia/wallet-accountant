package accountquery_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountquery"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
)

func TestQueryMediator_Account(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	expectedAccountId := account.IdFromUUID(uuid.New())
	notes := "some notes"
	expectedAccountEntity := accountreadmodel.Entity{
		AccountId:           expectedAccountId,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         common.Savings,
		StartingBalance:     306900,
		StartingBalanceDate: time.Now(),
		Currency:            account.CHF,
		Notes:               &notes,
		ActiveMonth: accountreadmodel.EntityActiveMonth{
			Month: time.March,
			Year:  2023,
		},
	}

	timesCalled := 0
	repositoryMock := accountreadmodel.ReadModelRepositoryMock{
		GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*accountreadmodel.Entity, error) {
			timesCalled++

			return &expectedAccountEntity, nil
		},
	}

	queryMediator := accountquery.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.Account(&ctx, expectedAccountId)
	requires.Nil(err)

	asserts.Equal(&expectedAccountEntity, actualAccount)

	asserts.Equal(1, timesCalled)
}

func TestQueryMediator_Accounts(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	expectedAccountId1 := account.IdFromUUID(uuid.New())
	notes1 := "some notes"
	expectedAccountEntity1 := accountreadmodel.Entity{
		AccountId:           expectedAccountId1,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         common.Savings,
		StartingBalance:     306900,
		StartingBalanceDate: time.Now(),
		Currency:            account.CHF,
		Notes:               &notes1,
		ActiveMonth: accountreadmodel.EntityActiveMonth{
			Month: time.March,
			Year:  2023,
		},
	}

	expectedAccountId2 := account.IdFromUUID(uuid.New())
	notes2 := "some notes 2"
	expectedAccountEntity2 := accountreadmodel.Entity{
		AccountId:           expectedAccountId2,
		BankName:            "bank name2",
		Name:                "account name2",
		AccountType:         common.Checking,
		StartingBalance:     406900,
		StartingBalanceDate: time.Now(),
		Currency:            account.USD,
		Notes:               &notes2,
		ActiveMonth: accountreadmodel.EntityActiveMonth{
			Month: time.April,
			Year:  2022,
		},
	}

	timesCalled := 0
	repositoryMock := accountreadmodel.ReadModelRepositoryMock{
		GetAllFn: func(ctx context.Context) ([]*accountreadmodel.Entity, error) {
			timesCalled++

			return []*accountreadmodel.Entity{
				&expectedAccountEntity1,
				&expectedAccountEntity2,
			}, nil
		},
	}

	queryMediator := accountquery.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccounts, err := queryMediator.Accounts(&ctx)
	requires.Nil(err)

	asserts.Equal(2, len(actualAccounts))
	asserts.Equal(&expectedAccountEntity1, actualAccounts[0])
	asserts.Equal(&expectedAccountEntity2, actualAccounts[1])

	asserts.Equal(1, timesCalled)
}

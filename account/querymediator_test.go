package account_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/account"
)

func TestQueryMediator_Account(t *testing.T) {
	asserts := assert.New(t)

	expectedAccountId := account.Id(uuid.New())
	expectedAccountEntity := account.Entity{
		AccountId:           &expectedAccountId,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         account.Savings,
		StartingBalance:     3069,
		StartingBalanceDate: time.Now(),
		Currency:            account.CHF,
		Notes:               "some notes",
		ActiveMonth: account.EntityActiveMonth{
			Month: time.March,
			Year:  2023,
		},
	}

	timesCalled := 0
	repositoryMock := account.ReadModelRepositoryMock{
		GetByAccountIdFn: func(ctx context.Context, accountId *account.Id) (*account.Entity, error) {
			timesCalled++

			return &expectedAccountEntity, nil
		},
	}

	queryMediator := account.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.Account(&ctx, &expectedAccountId)
	asserts.NoError(err)

	asserts.Equal(&expectedAccountEntity, actualAccount)

	asserts.Equal(1, timesCalled)
}

func TestQueryMediator_Accounts(t *testing.T) {
	asserts := assert.New(t)

	expectedAccountId1 := account.Id(uuid.New())
	expectedAccountEntity1 := account.Entity{
		AccountId:           &expectedAccountId1,
		BankName:            "bank name",
		Name:                "account name",
		AccountType:         account.Savings,
		StartingBalance:     3069,
		StartingBalanceDate: time.Now(),
		Currency:            account.CHF,
		Notes:               "some notes",
		ActiveMonth: account.EntityActiveMonth{
			Month: time.March,
			Year:  2023,
		},
	}

	expectedAccountId2 := account.Id(uuid.New())
	expectedAccountEntity2 := account.Entity{
		AccountId:           &expectedAccountId2,
		BankName:            "bank name2",
		Name:                "account name2",
		AccountType:         account.Checking,
		StartingBalance:     4069,
		StartingBalanceDate: time.Now(),
		Currency:            account.USD,
		Notes:               "some notes 2",
		ActiveMonth: account.EntityActiveMonth{
			Month: time.April,
			Year:  2022,
		},
	}

	timesCalled := 0
	repositoryMock := account.ReadModelRepositoryMock{
		GetAllFn: func(ctx context.Context) ([]*account.Entity, error) {
			timesCalled++

			return []*account.Entity{
				&expectedAccountEntity1,
				&expectedAccountEntity2,
			}, nil
		},
	}

	queryMediator := account.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccounts, err := queryMediator.Accounts(&ctx)
	asserts.NoError(err)

	asserts.Equal(2, len(actualAccounts))
	asserts.Equal(&expectedAccountEntity1, actualAccounts[0])
	asserts.Equal(&expectedAccountEntity2, actualAccounts[1])

	asserts.Equal(1, timesCalled)
}

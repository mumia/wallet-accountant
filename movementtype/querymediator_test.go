package movementtype_test

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"walletaccountant/movementtype"
)

func TestQueryMediator_MovementType(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	timesCalled := 0
	repositoryMock := movementtype.ReadModelRepositoryMock{
		GetByMovementTypeIdFn: func(ctx context.Context, accountId *movementtype.Id) (*movementtype.Entity, error) {
			timesCalled++

			return &movementTypeEntity1, nil
		},
	}

	queryMediator := movementtype.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccount, err := queryMediator.MovementType(&ctx, &movementTypeId1)
	requires.Nil(err)

	asserts.Equal(&movementTypeEntity1, actualAccount)

	asserts.Equal(1, timesCalled)
}

func TestQueryMediator_MovementTypes(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	timesCalled := 0
	repositoryMock := movementtype.ReadModelRepositoryMock{
		GetAllFn: func(ctx context.Context) ([]*movementtype.Entity, error) {
			timesCalled++

			return []*movementtype.Entity{
				&movementTypeEntity1,
				&movementTypeEntityWithSourceAccount,
			}, nil
		},
	}

	queryMediator := movementtype.NewQueryMediator(&repositoryMock)

	ctx := gin.Context{}
	actualAccounts, err := queryMediator.MovementTypes(&ctx)
	requires.Nil(err)

	asserts.Equal(2, len(actualAccounts))
	asserts.Equal(&movementTypeEntity1, actualAccounts[0])
	asserts.Equal(&movementTypeEntityWithSourceAccount, actualAccounts[1])

	asserts.Equal(1, timesCalled)
}

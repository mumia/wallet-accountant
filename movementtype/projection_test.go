package movementtype_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/movementtype"
	"walletaccountant/tagcategory"
)

func TestProjection_HandleEvent(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	movementTypeId := movementtype.Id(uuid.MustParse("72a196bc-d9b1-4c57-a916-3eabf1bf167b"))
	accountId := account.Id(uuid.MustParse("aeea307f-3c57-467c-8954-5f541aef6772"))
	sourceAccountId := account.Id(uuid.MustParse("f4081021-adf4-4b04-a6e5-4ad0028b96f9"))
	var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
	var notes1 = "my movement type notes"
	newMovementTypeRegisteredData := movementtype.NewMovementTypeRegisteredData{
		MovementTypeId:  &movementTypeId,
		Type:            movementtype.Credit,
		AccountId:       &accountId,
		SourceAccountId: &sourceAccountId,
		Description:     "movement type description",
		Notes:           &notes1,
		TagIds:          []*tagcategory.TagId{&tagId1},
	}

	expectedMovementTypeEntity := movementtype.Entity{
		MovementTypeId:  newMovementTypeRegisteredData.MovementTypeId,
		Type:            newMovementTypeRegisteredData.Type,
		AccountId:       newMovementTypeRegisteredData.AccountId,
		SourceAccountId: newMovementTypeRegisteredData.SourceAccountId,
		Description:     newMovementTypeRegisteredData.Description,
		Notes:           newMovementTypeRegisteredData.Notes,
		Tags:            newMovementTypeRegisteredData.TagIds,
	}

	createCallCount := 0
	repository := &movementtype.ReadModelRepositoryMock{
		CreateFn: func(ctx context.Context, actualMovementType *movementtype.Entity) error {
			createCallCount++

			asserts.Equal(&expectedMovementTypeEntity, actualMovementType)

			return nil
		},
	}

	projector, err := movementtype.NewProjection(repository)
	requires.NoError(err)

	newMovementTypeRegisteredEvent := eventhorizon.NewEvent(
		movementtype.NewMovementTypeRegistered,
		&newMovementTypeRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(movementtype.AggregateType, movementTypeId, 1),
	)

	err = projector.HandleEvent(context.Background(), newMovementTypeRegisteredEvent)
	requires.NoError(err)

	asserts.Equal(1, createCallCount)
}

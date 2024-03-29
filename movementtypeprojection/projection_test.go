package movementtypeprojection_test

import (
	"context"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
	"walletaccountant/movementtype"
	"walletaccountant/movementtypeprojection"
	"walletaccountant/movementtypereadmodel"
	"walletaccountant/tagcategory"
)

func TestProjection_HandleEvent(t *testing.T) {
	asserts := assert.New(t)
	requires := require.New(t)

	movementTypeId := movementtype.IdFromUUIDString("72a196bc-d9b1-4c57-a916-3eabf1bf167b")
	accountId := account.IdFromUUIDString("aeea307f-3c57-467c-8954-5f541aef6772")
	sourceAccountId := account.IdFromUUIDString("f4081021-adf4-4b04-a6e5-4ad0028b96f9")
	var tagId1 = tagcategory.TagId(uuid.MustParse("07a7ccde-b19c-412a-a054-bc09ac529357"))
	var notes1 = "my movement type notes"
	newMovementTypeRegisteredData := movementtype.NewMovementTypeRegisteredData{
		MovementTypeId:  movementTypeId,
		Action:          common.Credit,
		AccountId:       accountId,
		SourceAccountId: sourceAccountId,
		Description:     "movement type description",
		Notes:           &notes1,
		TagIds:          []*tagcategory.TagId{&tagId1},
	}

	expectedMovementTypeEntity := movementtypereadmodel.Entity{
		MovementTypeId:  newMovementTypeRegisteredData.MovementTypeId,
		Action:          newMovementTypeRegisteredData.Action,
		AccountId:       newMovementTypeRegisteredData.AccountId,
		SourceAccountId: newMovementTypeRegisteredData.SourceAccountId,
		Description:     newMovementTypeRegisteredData.Description,
		Notes:           newMovementTypeRegisteredData.Notes,
		Tags:            newMovementTypeRegisteredData.TagIds,
	}

	createCallCount := 0
	repository := &movementtypereadmodel.ReadModelRepositoryMock{
		CreateFn: func(ctx context.Context, actualMovementType *movementtypereadmodel.Entity) error {
			createCallCount++

			asserts.Equal(&expectedMovementTypeEntity, actualMovementType)

			return nil
		},
	}

	projector, err := movementtypeprojection.NewProjection(repository)
	requires.NoError(err)

	ctx, cancelCtx := context.WithCancel(context.Background())

	type eventCountStruct struct {
		count int
	}

	eventCount := &eventCountStruct{0}
	go func(eventCount *eventCountStruct) {
		keepRunning := true
		for keepRunning {
			select {
			case <-ctx.Done():
				keepRunning = false

			case <-projector.UpdateChannel():
				eventCount.count++
			}
		}
	}(eventCount)

	newMovementTypeRegisteredEvent := eventhorizon.NewEvent(
		movementtype.NewMovementTypeRegistered,
		&newMovementTypeRegisteredData,
		time.Now(),
		eventhorizon.ForAggregate(movementtype.AggregateType, *movementTypeId, 1),
	)

	err = projector.HandleEvent(context.Background(), newMovementTypeRegisteredEvent)
	requires.NoError(err)

	// Wait for all update channel messages to be processed
	time.Sleep(50 * time.Millisecond)
	cancelCtx()

	asserts.Equal(1, createCallCount)
	asserts.Equal(1, eventCount.count)
}

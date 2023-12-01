package movementtype_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/gofrs/uuid"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/movementtype"
	"walletaccountant/subscription"
)

func TestSubscribeEventStream(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()

	eventhorizon.RegisterEventData(
		movementtype.NewMovementTypeRegistered,
		func() eventhorizon.EventData { return &movementtype.NewMovementTypeRegisteredData{} },
	)
	defer eventhorizon.UnregisterEventData(movementtype.NewMovementTypeRegistered)

	var newMovementTypeRegistered = &esdb.PersistentSubscriptionEvent{
		EventAppeared: &esdb.EventAppeared{
			Event: &esdb.ResolvedEvent{
				Link: nil,
				Event: &esdb.RecordedEvent{
					EventID:        uuid.UUID{},
					EventType:      movementtype.NewMovementTypeRegistered.String(),
					ContentType:    "",
					StreamID:       "",
					EventNumber:    0,
					Position:       esdb.Position{},
					CreatedDate:    time.Now(),
					Data:           []byte("{}"),
					SystemMetadata: nil,
					UserMetadata:   nil,
				},
				Commit: nil,
			},
			RetryCount: 0,
		},
		SubscriptionDropped: nil,
		CheckPointReached:   nil,
	}

	var subscriptionReceiveChannel = make(chan bool)
	var subscriptionReceiveCalled = 0
	persistentSubscriptionMock := &eventstoredb.PersistentSubscriptionMock{
		RecvFn: func() *esdb.PersistentSubscriptionEvent {
			var event *esdb.PersistentSubscriptionEvent

			switch subscriptionReceiveCalled {
			case 0:
				event = newMovementTypeRegistered

			case 1:
				event = newMovementTypeRegistered

			case 2:
				event = &esdb.PersistentSubscriptionEvent{
					SubscriptionDropped: &esdb.SubscriptionDropped{
						Error: fmt.Errorf("subscription has been dropped"),
					},
				}

			default:
				subscriptionReceiveChannel <- true

				event = &esdb.PersistentSubscriptionEvent{
					SubscriptionDropped: &esdb.SubscriptionDropped{
						Error: fmt.Errorf("subscription has been dropped"),
					},
				}
			}

			subscriptionReceiveCalled++

			return event
		},
	}

	newMovementTypeRegisteredEventHandled := 0
	eventHandler := &mocks.EventHandlerMock{
		HandlerTypeFn: func() eventhorizon.EventHandlerType {
			return eventhorizon.EventHandlerType(movementtype.AggregateType)
		},
		HandleEventFn: func(ctx context.Context, event eventhorizon.Event) error {
			if event.EventType() == movementtype.NewMovementTypeRegistered {
				newMovementTypeRegisteredEventHandled++
			}

			if newMovementTypeRegisteredEventHandled > 1 {
				return errors.New("an error")
			}

			return nil
		},
	}

	subscription.SubscribeEventStreamTestHelper(
		ctx,
		t,
		movementtype.AggregateType,
		eventhorizon.EventHandlerType(movementtype.AggregateType),
		true,
		persistentSubscriptionMock,
		movementtype.NewProjectionConfig(eventHandler),
		subscriptionReceiveChannel,
	)

	expectedSubscriptionReceiveCalled := 4
	asserts.Equal(
		expectedSubscriptionReceiveCalled,
		subscriptionReceiveCalled,
		fmt.Sprintf("Subscription::Recv was expected to be called %d times", expectedSubscriptionReceiveCalled),
	)
	expectedNewMovementTypeRegisteredEventHandledEventHandled := 2
	asserts.Equal(
		expectedNewMovementTypeRegisteredEventHandledEventHandled,
		newMovementTypeRegisteredEventHandled,
		fmt.Sprintf(
			"New tag in new category add event was not handled %d times",
			expectedNewMovementTypeRegisteredEventHandledEventHandled,
		),
	)
}

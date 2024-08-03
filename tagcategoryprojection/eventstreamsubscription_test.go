package tagcategoryprojection_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
	"github.com/looplab/eventhorizon"
	"github.com/looplab/eventhorizon/uuid"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
	"walletaccountant/eventstoredb"
	"walletaccountant/mocks"
	"walletaccountant/subscription"
	"walletaccountant/tagcategory"
	"walletaccountant/tagcategoryprojection"
)

func TestSubscribeEventStream(t *testing.T) {
	asserts := assert.New(t)
	ctx := context.Background()

	eventhorizon.RegisterEventData(
		tagcategory.NewTagAddedToNewCategory,
		func() eventhorizon.EventData { return &tagcategory.NewTagAddedToNewCategoryData{} },
	)
	eventhorizon.RegisterEventData(
		tagcategory.NewTagAddedToExistingCategory,
		func() eventhorizon.EventData { return &tagcategory.NewTagAddedToExistingCategoryData{} },
	)
	defer eventhorizon.UnregisterEventData(tagcategory.NewTagAddedToNewCategory)
	defer eventhorizon.UnregisterEventData(tagcategory.NewTagAddedToExistingCategory)

	var newTagAddedToNewCategory = &esdb.PersistentSubscriptionEvent{
		EventAppeared: &esdb.EventAppeared{
			Event: &esdb.ResolvedEvent{
				Link: nil,
				Event: &esdb.RecordedEvent{
					EventID:        uuid.UUID{},
					EventType:      tagcategory.NewTagAddedToNewCategory.String(),
					ContentType:    "",
					StreamID:       "tagCategory-dab6a78e-3c19-49b1-8dde-8ed974f964ac",
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

	var newTagAddedToExistingCategory = &esdb.PersistentSubscriptionEvent{
		EventAppeared: &esdb.EventAppeared{
			Event: &esdb.ResolvedEvent{
				Link: nil,
				Event: &esdb.RecordedEvent{
					EventID:        uuid.UUID{},
					EventType:      tagcategory.NewTagAddedToExistingCategory.String(),
					ContentType:    "",
					StreamID:       "tagCategory-cd5c7ffc-87bc-493d-8e60-ad99c2e963bd",
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
				event = newTagAddedToNewCategory

			case 1:
				event = newTagAddedToExistingCategory

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

	newTagAddedToNewCategoryEventHandled := 0
	newTagAddedToExistingCategoryEventHandled := 0
	eventHandler := &mocks.EventHandlerMock{
		HandlerTypeFn: func() eventhorizon.EventHandlerType {
			return eventhorizon.EventHandlerType(tagcategory.AggregateType)
		},
		HandleEventFn: func(ctx context.Context, event eventhorizon.Event) error {
			if event.EventType() == tagcategory.NewTagAddedToNewCategory {
				newTagAddedToNewCategoryEventHandled++
			} else if event.EventType() == tagcategory.NewTagAddedToExistingCategory {
				newTagAddedToExistingCategoryEventHandled++
			}

			if newTagAddedToExistingCategoryEventHandled > 0 {
				return errors.New("an error")
			}

			return nil
		},
	}

	subscription.SubscribeEventStreamTestHelper(
		ctx,
		t,
		tagcategory.AggregateType,
		eventhorizon.EventHandlerType(tagcategory.AggregateType),
		true,
		persistentSubscriptionMock,
		tagcategoryprojection.NewProjectionConfig(eventHandler),
		subscriptionReceiveChannel,
	)

	expectedSubscriptionReceiveCalled := 4
	asserts.Equal(
		expectedSubscriptionReceiveCalled,
		subscriptionReceiveCalled,
		fmt.Sprintf("Subscription::Recv was expected to be called %d times", expectedSubscriptionReceiveCalled),
	)
	expectedNewTagAddedToNewCategoryEventHandled := 1
	asserts.Equal(
		expectedNewTagAddedToNewCategoryEventHandled,
		newTagAddedToExistingCategoryEventHandled,
		fmt.Sprintf(
			"New tag in new category add event was not handled %d times",
			expectedNewTagAddedToNewCategoryEventHandled,
		),
	)

	expectedNewTagAddedToExistingCategoryEventHandled := 1
	asserts.Equal(
		expectedNewTagAddedToExistingCategoryEventHandled,
		newTagAddedToExistingCategoryEventHandled,
		fmt.Sprintf(
			"New tag in existing category add event was not handled %d times",
			expectedNewTagAddedToExistingCategoryEventHandled,
		),
	)
}

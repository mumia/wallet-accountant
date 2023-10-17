package tagcategory_test

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
	"walletaccountant/subscription"
	"walletaccountant/tagcategory"
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
					StreamID:       "",
					EventNumber:    0,
					Position:       esdb.Position{},
					CreatedDate:    time.Now(),
					Data:           []byte("{\"tagCategoryId\": \"aeea307f-3c57-467c-8954-5f541aef6772\", \"TagCategoryName\": \"locations1\", \"TagCategoryNotes\": \"My locations\", \"tagId\": \"07a7ccde-b19c-412a-a054-bc09ac529357\", \"tagName\": \"Vieira\", \"tagNotes\": \"Tugalandia\"}"),
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
					StreamID:       "",
					EventNumber:    0,
					Position:       esdb.Position{},
					CreatedDate:    time.Now(),
					Data:           []byte("{\n  \"tagCategoryId\": \"aeea307f-3c57-467c-8954-5f541aef6772\",\n  \"tagId\": \"5c6e143d-f1a6-42ca-b9df-2f4a94628194\",\n  \"name\": \"Berlin\",\n  \"notes\": \"Germania\"\n}"),
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
		persistentSubscriptionMock,
		tagcategory.NewProjectionConfig(eventHandler),
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

package subscription

import (
	"context"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/eventstoredb"
	"walletaccountant/projector"
)

func SubscribeEventStreamTestHelper(
	ctx context.Context,
	t *testing.T,
	aggregateType eventhorizon.AggregateType,
	subscriptionMock *eventstoredb.PersistentSubscriptionMock,
	projection eventhorizon.EventHandler,
	subscriptionFinishedChannel chan bool,
) {
	asserts := assert.New(t)
	requires := require.New(t)

	projectionStream := fmt.Sprintf("$ce-%s", aggregateType)
	subscriptionGroup := fmt.Sprintf("subscription-group-%s", aggregateType)

	lifecycle := fxtest.NewLifecycle(t)

	var subscriptionCreateCalled = false
	var subscriptionReceiveCalled = false
	var subscriptionAckCalled = false
	var subscriptionNackCalled = false
	var subscriptionCloseCalled = false
	client := &eventstoredb.ClientMock{
		SubscribeToPersistentSubscriptionFn: func(
			ctx context.Context,
			streamName string,
			groupName string,
			options esdb.SubscribeToPersistentSubscriptionOptions,
		) (eventstoredb.PersistentSubscriptioner, error) {
			return &eventstoredb.PersistentSubscriptionMock{
				RecvFn: func() *esdb.PersistentSubscriptionEvent {
					subscriptionReceiveCalled = true

					if subscriptionMock.RecvFn != nil {
						return subscriptionMock.RecvFn()
					}

					return nil
				},
				AckFn: func(messages ...*esdb.ResolvedEvent) error {
					subscriptionAckCalled = true

					if subscriptionMock.AckFn != nil {
						return subscriptionMock.AckFn(messages...)
					}

					return nil
				},
				NackFn: func(reason string, action esdb.NackAction, messages ...*esdb.ResolvedEvent) error {
					subscriptionNackCalled = true

					if subscriptionMock.NackFn != nil {
						return subscriptionMock.NackFn(reason, action, messages...)
					}

					return nil
				},
				CloseFn: func() error {
					subscriptionCloseCalled = true

					if subscriptionMock.CloseFn != nil {
						return subscriptionMock.CloseFn()
					}

					return nil
				},
			}, nil
		},

		CreatePersistentSubscriptionFn: func(
			ctx context.Context,
			streamName string,
			groupName string,
			options esdb.PersistentStreamSubscriptionOptions,
		) error {
			subscriptionCreateCalled = true
			asserts.Equal(projectionStream, streamName)
			asserts.Equal(subscriptionGroup, groupName)

			return nil
		},
	}

	projectionConfig := account.NewProjectionConfig(projection)
	eventMatcherHandlerRegistry, err := projector.NewEventMatcherHandlerRegistry(
		[]definitions.EventMatcherHandleProvider{projectionConfig},
	)
	requires.NoError(err)

	var startCalled = false
	var stopCalled = false
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error { startCalled = true; return nil },
		OnStop:  func(context.Context) error { stopCalled = true; return nil },
	})

	err = account.SubscribeEventStream(
		client,
		eventMatcherHandlerRegistry,
		zaptest.NewLogger(t),
		lifecycle,
	)
	requires.NoError(err)

	requires.NoError(lifecycle.Start(ctx))
	asserts.True(startCalled, "lifecycle start was never called")

	keepRunning := true
	for {
		select {
		case <-subscriptionFinishedChannel:
			keepRunning = false
		case <-ctx.Done():
			keepRunning = false
		case <-time.After(1 * time.Second):
			keepRunning = false
		}

		if !keepRunning {
			break
		}
	}

	requires.NoError(lifecycle.Stop(ctx))
	asserts.True(stopCalled, "lifecycle stop was never called")

	asserts.True(subscriptionCreateCalled, "CreatePersistentSubscription was never called")
	asserts.True(subscriptionReceiveCalled, "Subscription::Recv was never called")
	asserts.True(subscriptionAckCalled, "Subscription::Ack was never called")
	asserts.True(subscriptionNackCalled, "Subscription::Nack was never called")
	asserts.True(subscriptionCloseCalled, "Subscription::Close was never called")
}

package account

import (
	"context"
	"errors"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"github.com/looplab/eventhorizon"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"time"
	"walletaccountant/eventstoredb"
	"walletaccountant/projector"
)

var deadlineTimeout = 1 * time.Minute
var projectionStream = fmt.Sprintf("$ce-%s", AggregateType)
var subscriptionGroup = fmt.Sprintf("subscription-group-%s", AggregateType)

type EventStreamSubscription struct {
	client                      eventstoredb.EventStorerer
	eventMatcher                eventhorizon.EventMatcher
	eventHandler                eventhorizon.EventHandler
	eventMatcherHandlerRegistry *projector.EventMatcherHandlerRegistry
	logger                      *zap.Logger
}

func SubscribeEventStream(
	client eventstoredb.EventStorerer,
	eventMatcherHandlerRegistry *projector.EventMatcherHandlerRegistry,
	logger *zap.Logger,
	lifecycle fx.Lifecycle,
) error {
	eventMatcher, eventHandler, err := eventMatcherHandlerRegistry.GetHandler(
		eventhorizon.EventHandlerType(AggregateType),
	)
	if err != nil {
		return fmt.Errorf(
			"failed to match an event handler. EventHandlerType: %s Error: %w",
			AggregateType,
			err,
		)
	}

	subscription := &EventStreamSubscription{
		client:       client,
		eventMatcher: eventMatcher,
		eventHandler: eventHandler,
		logger:       logger,
	}

	var subscriptionLifecycleCtx context.Context
	var subscriptionLifecycleCtxCancel context.CancelFunc
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			subscriptionLifecycleCtx, subscriptionLifecycleCtxCancel = context.WithCancel(context.Background())

			if err := subscription.createSubscription(); err != nil {
				return err
			}

			go subscription.handleSubscription(subscriptionLifecycleCtx)

			return nil
		},

		OnStop: func(ctx context.Context) error {
			subscriptionLifecycleCtxCancel()

			return nil
		},
	})

	return nil
}

func (ess EventStreamSubscription) handleSubscription(ctx context.Context) {
	for {
		subscriptionCtx, cancelFunc := context.WithCancel(ctx)

		subscription, err := ess.client.SubscribeToPersistentSubscription(
			subscriptionCtx,
			projectionStream,
			subscriptionGroup,
			esdb.SubscribeToPersistentSubscriptionOptions{
				Deadline: &deadlineTimeout,
			},
		)
		if err != nil {
			// TODO: we should retry

			ess.logger.Error(
				fmt.Errorf(
					"failed to subscribe to stream. Stream: %s Group: %s Error: %w",
					projectionStream,
					subscriptionGroup,
					err,
				).Error(),
			)

			cancelFunc()

			return
		}

		_, ok := subscription.(eventstoredb.PersistentSubscriptioner)
		if !ok {
			ess.logger.Error(fmt.Errorf("subscription is not of type esdb.PersistentSubscription").Error())

			cancelFunc()

			return
		}

		select {
		case <-subscriptionCtx.Done():
			if err := subscriptionCtx.Err(); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					continue
				}

				ess.closeSubscription(subscription)

				fx.Error(err)

				cancelFunc()

				return
			}

			break

		default:
			if err := ess.subscribe(subscriptionCtx, subscription); err != nil {
				//isDeadlineError := strings.Contains(err.Error(), "code = DeadlineExceeded")
				if !errors.Is(err, context.DeadlineExceeded) {
					ess.closeSubscription(subscription)

					fx.Error(err)

					break
				}
			}
		}
	}
}

func (ess EventStreamSubscription) closeSubscription(subscription eventstoredb.PersistentSubscriptioner) {
	if err := subscription.Close(); err != nil {
		ess.logger.Error(
			fmt.Sprintf(
				"failed to clode subscrition. Stream: %s Group: %s",
				projectionStream,
				subscriptionGroup,
			),
		)
	}
}

func (ess EventStreamSubscription) createSubscription() error {
	settings := esdb.SubscriptionSettingsDefault()
	settings.ResolveLinkTos = true

	err := ess.client.CreatePersistentSubscription(
		context.Background(),
		projectionStream,
		subscriptionGroup,
		esdb.PersistentStreamSubscriptionOptions{Settings: &settings},
	)

	if err != nil {
		// ignore if subscription already exists
		if esdbError, ok := err.(*esdb.Error); ok && esdbError.Code() == esdb.ErrorCodeResourceAlreadyExists {
			return nil
		}
	}

	return err
}

func (ess EventStreamSubscription) subscribe(
	subscriptionCtx context.Context,
	subscription eventstoredb.PersistentSubscriptioner,
) error {
	ess.logger.Info(
		fmt.Sprintf("start persistent subscription. Stream: %s Group: %s", projectionStream, subscriptionGroup),
	)

	for {
		event := subscription.Recv()
		ess.logger.Debug(
			fmt.Sprintf(
				"persistent subscription got new event. Stream: %s Group: %s",
				projectionStream,
				subscriptionGroup,
			),
		)

		select {
		case <-subscriptionCtx.Done():
			err := subscriptionCtx.Err()
			if err == context.DeadlineExceeded {
				continue
			}

			return err
		default:
		}

		if err := ess.checkDroppedSubscription(subscription, event); err != nil {
			return err
		}

		if event.EventAppeared == nil {
			continue
		}

		ess.logger.Debug(
			fmt.Sprintf(
				"persistent subscription processing new event. Event: %s Stream: %s Group: %s",
				event.EventAppeared.Event.Event.EventType,
				projectionStream,
				subscriptionGroup,
			),
		)

		toHandleEvent, err := eventstoredb.CreateEvent(event.EventAppeared.Event)
		if err != nil {
			ess.nackRetry(
				subscription,
				fmt.Errorf("failed to create event. Error: %w", err).Error(),
				event.EventAppeared.Event,
			)

			continue
		}

		esdbPersistentSubscription, ok := subscription.(eventstoredb.PersistentSubscriptioner)
		if !ok {
			err := fmt.Errorf("subscription is not of type esdb.PersistentSubscription")
			ess.logger.Error(err.Error())

			return err
		}

		if !ess.eventMatcher.Match(toHandleEvent) {
			ess.ack(esdbPersistentSubscription, event.EventAppeared.Event)

			continue
		}

		err = ess.eventHandler.HandleEvent(subscriptionCtx, toHandleEvent)
		if err != nil {
			ess.nackRetry(
				subscription,
				fmt.Errorf("failed to handle event. Error: %w", err).Error(),
				event.EventAppeared.Event,
			)

			continue
		}

		ess.ack(esdbPersistentSubscription, event.EventAppeared.Event)
	}
}

func (ess EventStreamSubscription) ack(
	subscription eventstoredb.PersistentSubscriptioner,
	event *esdb.ResolvedEvent,
) {
	if err := subscription.Ack(event); err != nil {
		ess.logger.Error(
			fmt.Errorf("failed to ACK event. Error: %w", err).Error(),
		)
	}
}

func (ess EventStreamSubscription) nackRetry(
	subscription eventstoredb.PersistentSubscriptioner,
	errorMessage string,
	event *esdb.ResolvedEvent,
) {
	ess.logger.Error(errorMessage)

	if err := subscription.Nack(errorMessage, esdb.NackActionRetry, event); err != nil {
		ess.logger.Error(
			fmt.Errorf("failed to NACK event. Error: %w", err).Error(),
		)
	}
}

func (ess EventStreamSubscription) checkDroppedSubscription(
	subscription eventstoredb.PersistentSubscriptioner,
	event *esdb.PersistentSubscriptionEvent,
) error {
	if event.SubscriptionDropped != nil {
		if event.EventAppeared != nil {
			ess.nackRetry(
				subscription,
				fmt.Errorf(
					"subscription dropped. Stream: %s Group: %s Error: %w",
					projectionStream,
					subscriptionGroup,
					event.SubscriptionDropped.Error,
				).Error(),
				event.EventAppeared.Event,
			)
		}

		return event.SubscriptionDropped.Error
	}

	return nil
}

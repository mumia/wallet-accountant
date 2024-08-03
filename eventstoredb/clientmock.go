package eventstoredb

import (
	"context"
	"github.com/EventStore/EventStore-Client-Go/v4/esdb"
)

var _ EventStorerer = &ClientMock{}

type ClientMock struct {
	AppendToStreamFn func(
		ctx context.Context,
		streamID string,
		opts esdb.AppendToStreamOptions,
		events ...esdb.EventData,
	) (*esdb.WriteResult, error)
	ReadStreamFn func(
		ctx context.Context,
		streamID string,
		opts esdb.ReadStreamOptions,
		count uint64,
	) (*esdb.ReadStream, error)
	CreatePersistentSubscriptionFn func(
		ctx context.Context,
		streamName string,
		groupName string,
		options esdb.PersistentStreamSubscriptionOptions,
	) error
	SubscribeToPersistentSubscriptionFn func(
		ctx context.Context,
		streamName string,
		groupName string,
		options esdb.SubscribeToPersistentSubscriptionOptions,
	) (PersistentSubscriptioner, error)
	CloseFn func() error
}

func (mock *ClientMock) AppendToStream(
	ctx context.Context,
	streamID string,
	opts esdb.AppendToStreamOptions,
	events ...esdb.EventData,
) (*esdb.WriteResult, error) {
	if mock != nil && mock.AppendToStreamFn != nil {
		return mock.AppendToStreamFn(ctx, streamID, opts, events...)
	}

	return nil, nil
}

func (mock *ClientMock) ReadStream(
	ctx context.Context,
	streamID string,
	opts esdb.ReadStreamOptions,
	count uint64,
) (*esdb.ReadStream, error) {
	if mock != nil && mock.ReadStreamFn != nil {
		return mock.ReadStreamFn(ctx, streamID, opts, count)
	}

	return nil, nil
}

func (mock *ClientMock) CreatePersistentSubscription(
	ctx context.Context,
	streamName string,
	groupName string,
	options esdb.PersistentStreamSubscriptionOptions,
) error {
	if mock != nil && mock.CreatePersistentSubscriptionFn != nil {
		return mock.CreatePersistentSubscriptionFn(ctx, streamName, groupName, options)
	}

	return nil
}

func (mock *ClientMock) SubscribeToPersistentSubscription(
	ctx context.Context,
	streamName string,
	groupName string,
	options esdb.SubscribeToPersistentSubscriptionOptions,
) (PersistentSubscriptioner, error) {
	if mock != nil && mock.SubscribeToPersistentSubscriptionFn != nil {
		return mock.SubscribeToPersistentSubscriptionFn(ctx, streamName, groupName, options)
	}

	return nil, nil
}

func (mock *ClientMock) Close() error {
	if mock != nil && mock.CloseFn != nil {
		return mock.CloseFn()
	}

	return nil
}

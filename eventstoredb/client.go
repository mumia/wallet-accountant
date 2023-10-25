package eventstoredb

import (
	"context"
	"fmt"
	"github.com/EventStore/EventStore-Client-Go/v3/esdb"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
)

const connectionStringName = "EVENTSTORE_CONNETION_STRING"

var _ EventStorerer = &Client{}

type EventWriter interface {
	AppendToStream(
		context context.Context,
		streamID string,
		opts esdb.AppendToStreamOptions,
		events ...esdb.EventData,
	) (*esdb.WriteResult, error)
}

type EventReader interface {
	ReadStream(
		context context.Context,
		streamID string,
		opts esdb.ReadStreamOptions,
		count uint64,
	) (*esdb.ReadStream, error)
}

type EventSubscriber interface {
	CreatePersistentSubscription(
		ctx context.Context,
		streamName string,
		groupName string,
		options esdb.PersistentStreamSubscriptionOptions,
	) error
	SubscribeToPersistentSubscription(
		ctx context.Context,
		streamName string,
		groupName string,
		options esdb.SubscribeToPersistentSubscriptionOptions,
	) (PersistentSubscriptioner, error)
}

type EventStorerer interface {
	EventWriter
	EventReader
	EventSubscriber
	Close() error
}

// Client Wraps esdb.Client to simplify mocking
type Client struct {
	client *esdb.Client
}

func NewClient(log *zap.Logger) (*Client, error) {
	configuration, err := esdb.ParseConnectionString(os.Getenv(connectionStringName))
	if err != nil {
		return nil, err
	}

	configuration.Logger = func(level esdb.LogLevel, format string, args ...interface{}) {
		logMessage := fmt.Sprintf(format, args...)

		var logLevel zapcore.Level
		switch level {
		case esdb.LogDebug:
			logLevel = zapcore.DebugLevel

		case esdb.LogInfo:
			logLevel = zapcore.InfoLevel

		case esdb.LogWarn:
			logLevel = zapcore.WarnLevel

		case esdb.LogError:
			logLevel = zapcore.ErrorLevel

			// Used for the event subscription group creation error message
			if strings.Contains(logMessage, "code = AlreadyExists") {
				logLevel = zapcore.InfoLevel
			}
		}

		log.Log(logLevel, logMessage)
	}

	client, err := esdb.NewClient(configuration)
	if err != nil {
		return nil, err
	}

	return &Client{client: client}, nil
}

func (c Client) AppendToStream(
	context context.Context,
	streamID string,
	opts esdb.AppendToStreamOptions,
	events ...esdb.EventData,
) (*esdb.WriteResult, error) {
	return c.client.AppendToStream(context, streamID, opts, events...)
}

func (c Client) ReadStream(
	context context.Context,
	streamID string,
	opts esdb.ReadStreamOptions,
	count uint64,
) (*esdb.ReadStream, error) {
	return c.client.ReadStream(context, streamID, opts, count)
}

func (c Client) CreatePersistentSubscription(
	ctx context.Context,
	streamName string,
	groupName string,
	options esdb.PersistentStreamSubscriptionOptions,
) error {
	return c.client.CreatePersistentSubscription(ctx, streamName, groupName, options)
}

func (c Client) SubscribeToPersistentSubscription(
	ctx context.Context,
	streamName string,
	groupName string,
	options esdb.SubscribeToPersistentSubscriptionOptions,
) (PersistentSubscriptioner, error) {
	return c.client.SubscribeToPersistentSubscription(ctx, streamName, groupName, options)
}

func (c Client) Close() error {
	return c.client.Close()
}

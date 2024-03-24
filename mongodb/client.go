package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
	"os"
)

const connectionStringName = "MONGO_CONNECTION_STRING"
const DatabaseName = "wallet_accountant"

type MongoClient struct {
	*mongo.Client
	logger *zap.Logger
}

func NewMongoClient(logger *zap.Logger) (*MongoClient, error) {
	logger = logger.With(zap.String("tag", "mongoclient"))

	cmdMonitor := &event.CommandMonitor{
		Started: func(_ context.Context, evt *event.CommandStartedEvent) {
			logger.Debug(evt.Command.String())
		},
	}

	opts := options.Client().ApplyURI(os.Getenv(connectionStringName))
	opts.SetWriteConcern(writeconcern.Majority())
	opts.SetReadConcern(readconcern.Majority())
	opts.SetReadPreference(readpref.Primary())
	opts.SetMonitor(cmdMonitor)

	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Mongo DB: %w", err)
	}

	return &MongoClient{client, logger}, nil
}

func (client *MongoClient) Database() *mongo.Database {
	return client.Client.Database(DatabaseName)
}

func (client *MongoClient) Collection(collectionName string) *mongo.Collection {
	return client.Database().Collection(collectionName)
}

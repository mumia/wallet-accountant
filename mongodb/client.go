package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"os"
)

const connectionStringName = "MONGO_CONNECTION_STRING"
const DatabaseName = "wallet_accountant"

type MongoClient struct {
	*mongo.Client
}

func NewMongoClient() (*MongoClient, error) {
	opts := options.Client().ApplyURI(os.Getenv(connectionStringName))
	opts.SetWriteConcern(writeconcern.Majority())
	opts.SetReadConcern(readconcern.Majority())
	opts.SetReadPreference(readpref.Primary())
	//opts.SetRegistry(NewRegistry())

	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("could not connect to Mongo DB: %w", err)
	}

	return &MongoClient{client}, nil
}

func (client *MongoClient) Database() *mongo.Database {
	return client.Client.Database(DatabaseName)
}

func (client *MongoClient) Collection(collectionName string) *mongo.Collection {
	return client.Database().Collection(collectionName)
}

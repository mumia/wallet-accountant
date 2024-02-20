package accountreadmodel

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"walletaccountant/account"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	Create(ctx context.Context, account Entity) error
	UpdateActiveMonth(
		ctx context.Context,
		accountId *account.Id,
		activeMonth EntityActiveMonth,
	) error
}

type ReadModelReader interface {
	GetAll(ctx context.Context) ([]*Entity, error)
	GetByAccountId(ctx context.Context, accountId *account.Id) (*Entity, error)
	GetByName(ctx context.Context, name string) (*Entity, error)
}

type ReadModeler interface {
	ReadModelWriter
	ReadModelReader
}

type ReadModelRepository struct {
	client *mongodb.MongoClient
}

func NewReadModelRepository(client *mongodb.MongoClient) *ReadModelRepository {
	return &ReadModelRepository{client: client}
}

func (repository *ReadModelRepository) Create(ctx context.Context, account Entity) error {
	_, err := repository.collection().ReplaceOne(
		ctx,
		bson.D{{"_id", account.AccountId}},
		account,
		options.Replace().SetUpsert(true),
	)

	return err
}

func (repository *ReadModelRepository) UpdateActiveMonth(
	ctx context.Context,
	accountId *account.Id,
	activeMonth EntityActiveMonth,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.D{{"_id", accountId}},
		bson.D{{"$set", bson.D{{"active_month", activeMonth}}}},
	)

	return err
}

func (repository *ReadModelRepository) GetAll(ctx context.Context) ([]*Entity, error) {
	cursor, err := repository.collection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	var entities []*Entity

	for cursor.Next(ctx) {
		var entity *Entity

		if err := cursor.Decode(&entity); err != nil {
			return nil, err
		}

		entities = append(entities, entity)
	}

	if err := cursor.Close(ctx); err != nil {
		return nil, err
	}

	return entities, nil
}

func (repository *ReadModelRepository) GetByAccountId(ctx context.Context, accountId *account.Id) (*Entity, error) {
	var entity *Entity

	err := repository.collection().FindOne(ctx, bson.M{"_id": accountId}).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) GetByName(ctx context.Context, name string) (*Entity, error) {
	var entity *Entity

	err := repository.collection().FindOne(ctx, bson.M{"name": name}).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(account.AggregateType.String())
}

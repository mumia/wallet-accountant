package movementtype

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"walletaccountant/account"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	Create(ctx context.Context, movementType *Entity) error
}

type ReadModelReader interface {
	GetAll(ctx context.Context) ([]*Entity, error)
	GetByMovementTypeId(ctx context.Context, movementTypeId *Id) (*Entity, error)
	GetByAccountId(ctx context.Context, accountId *account.Id) ([]*Entity, error)
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

func (repository *ReadModelRepository) Create(ctx context.Context, movementType *Entity) error {
	_, err := repository.collection().InsertOne(ctx, movementType)

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

func (repository *ReadModelRepository) GetByMovementTypeId(ctx context.Context, movementTypeId *Id) (*Entity, error) {
	var entity *Entity

	err := repository.collection().FindOne(ctx, bson.M{"_id": movementTypeId}).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) GetByAccountId(ctx context.Context, accountId *account.Id) ([]*Entity, error) {
	cursor, err := repository.collection().Find(ctx, bson.M{"account_id": accountId})
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

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(AggregateType.String())
}

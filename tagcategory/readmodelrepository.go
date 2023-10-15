package tagcategory

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	AddNewTagAndCategory(ctx context.Context, newTagAndCategory *CategoryEntity) error
	AddNewTagToCategory(ctx context.Context, categoryId *CategoryId, newTag *Entity) error
}

type ReadModelReader interface {
	ExistsByName(ctx context.Context, name string) (bool, error)
	GetAll(ctx context.Context) ([]*CategoryEntity, error)
	CategoryExistsById(ctx context.Context, id *CategoryId) (bool, error)
	CategoryExistsByName(ctx context.Context, name string) (bool, error)
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

func (repository *ReadModelRepository) AddNewTagAndCategory(
	ctx context.Context,
	newTagAndCategory *CategoryEntity,
) error {
	_, err := repository.collection().InsertOne(ctx, newTagAndCategory)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ReadModelRepository) AddNewTagToCategory(
	ctx context.Context,
	categoryId *CategoryId,
	newTag *Entity,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": categoryId},
		bson.M{"$push": bson.M{"tags": newTag}},
	)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ReadModelRepository) ExistsByName(ctx context.Context, name string) (bool, error) {
	err := repository.collection().FindOne(ctx, bson.M{"tags.name": name}).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = nil
		}

		return false, err
	}

	return true, nil
}

func (repository *ReadModelRepository) GetAll(ctx context.Context) ([]*CategoryEntity, error) {
	cursor, err := repository.collection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	var entities []*CategoryEntity

	for cursor.Next(ctx) {
		var entity *CategoryEntity

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

func (repository *ReadModelRepository) CategoryExistsById(ctx context.Context, id *CategoryId) (bool, error) {
	err := repository.collection().FindOne(ctx, bson.M{"_id": id}).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = nil
		}

		return false, err
	}

	return true, nil
}

func (repository *ReadModelRepository) CategoryExistsByName(ctx context.Context, name string) (bool, error) {
	err := repository.collection().FindOne(ctx, bson.M{"name": name}).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = nil
		}

		return false, err
	}

	return true, nil
}

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(AggregateType.String())
}

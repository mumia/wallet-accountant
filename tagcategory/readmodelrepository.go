package tagcategory

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	AddNewTagAndCategory(ctx context.Context, newTagAndCategory *CategoryEntity) error
	AddNewTagToCategory(ctx context.Context, categoryId *Id, newTag *Entity) error
}

type ReadModelReader interface {
	ExistsById(ctx context.Context, tagId *TagId) (bool, error)
	ExistsByName(ctx context.Context, name string) (bool, error)
	GetAll(ctx context.Context) ([]*CategoryEntity, error)
	GetByTagIds(ctx context.Context, tagIds []TagId) ([]*CategoryEntity, error)
	CategoryExistsById(ctx context.Context, id *Id) (bool, error)
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
	categoryId *Id,
	newTag *Entity,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": categoryId},
		bson.M{"$addToSet": bson.M{"tags": newTag}},
	)
	if err != nil {
		return err
	}

	return nil
}

func (repository *ReadModelRepository) ExistsById(ctx context.Context, tagId *TagId) (bool, error) {
	err := repository.collection().FindOne(ctx, bson.M{"tags._id": tagId}).Err()
	if err != nil {
		if err == mongo.ErrNoDocuments {
			err = nil
		}

		return false, err
	}

	return true, nil
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

func (repository *ReadModelRepository) GetByTagIds(ctx context.Context, tagIds []TagId) ([]*CategoryEntity, error) {
	findOptions := &options.FindOptions{}

	if len(tagIds) > 0 {
		findOptions.Projection = bson.M{
			"name":  1,
			"notes": 1,
			"tags": bson.M{
				"$filter": bson.D{
					{"input", "$tags"},
					{"as", "tags"},
					{"cond", bson.D{bson.E{"$in", bson.A{"$$tags._id", tagIds}}}},
				},
			},
		}
	}

	cursor, err := repository.collection().Find(
		ctx,
		bson.M{"tags._id": bson.M{"$in": tagIds}},
		findOptions,
	)
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

func (repository *ReadModelRepository) CategoryExistsById(ctx context.Context, id *Id) (bool, error) {
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
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = nil
		}

		return false, err
	}

	return true, nil
}

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(AggregateType.String())
}

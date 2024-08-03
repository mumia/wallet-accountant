package importfilereadmodel

import (
	"context"
	"errors"
	"github.com/looplab/eventhorizon/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	Register(ctx context.Context, importFile Entity) error
	StartParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	RestartParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	EndParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	FailParse(ctx context.Context, importFileId *importfile.Id, date time.Time, code string, reason string) error
	AddFileDataRow(ctx context.Context, importFileId *importfile.Id, dataRow FileRowEntity) error
	VerifyDataRow(ctx context.Context, dataRowId *importfile.DataRowId) error
	InvalidateDataRow(ctx context.Context, dataRowId *importfile.DataRowId) error
}

type ReadModelReader interface {
	GetAll(ctx context.Context) ([]*Entity, error)
	GetById(ctx context.Context, importFileId *importfile.Id) (*Entity, error)
	GetFileRowsById(ctx context.Context, importFileId *importfile.Id) (*FileRowsEntity, error)
	GetFileRowByRowId(ctx context.Context, FileDataRowId *importfile.DataRowId) (*FileRowEntity, error)
	GetByAccountId(ctx context.Context, accountId *account.Id) ([]*Entity, error)
	GetAndLockNextFileToParse(ctx context.Context) (*Entity, error)
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

func (repository *ReadModelRepository) Register(ctx context.Context, importFile Entity) error {
	importFile.State = importfile.Imported

	_, err := repository.collection().ReplaceOne(
		ctx,
		bson.M{"_id": importFile.ImportFileId},
		importFile,
		options.Replace().SetUpsert(true),
	)

	return err
}

func (repository *ReadModelRepository) StartParse(
	ctx context.Context,
	importFileId *importfile.Id,
	date time.Time,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": importFileId},
		bson.M{
			"$set": bson.M{
				"state":            importfile.ParsingStarted,
				"start_parse_date": date,
			},
		},
	)

	return err
}

func (repository *ReadModelRepository) RestartParse(
	ctx context.Context,
	importFileId *importfile.Id,
	date time.Time,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": importFileId},
		bson.M{
			"$set": bson.M{
				"state":            importfile.ParsingRestarted,
				"start_parse_date": date,
			},
		},
	)

	return err
}

func (repository *ReadModelRepository) EndParse(
	ctx context.Context,
	importFileId *importfile.Id,
	date time.Time,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": importFileId},
		bson.M{
			"$set": bson.D{
				{"state", importfile.ParsingEnded},
				{"end_parse_date", date},
			},
			"$unset": bson.M{"locked_until": ""},
		},
	)

	return err
}

func (repository *ReadModelRepository) FailParse(
	ctx context.Context,
	importFileId *importfile.Id,
	date time.Time,
	code string,
	reason string,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": importFileId},
		bson.M{
			"$set": bson.D{
				{"state", importfile.ParsingFailed},
				{"fail_parse_date", date},
				{"code", code},
				{"reason", reason},
			},
			"$unset": bson.M{"locked_until": ""},
		},
	)

	return err
}

func (repository *ReadModelRepository) AddFileDataRow(
	ctx context.Context,
	importFileId *importfile.Id,
	dataRow FileRowEntity,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": importFileId},
		bson.M{
			"$inc":      bson.M{"row_count": 1},
			"$addToSet": bson.M{"rows": dataRow},
		},
	)

	return err
}

func (repository *ReadModelRepository) VerifyDataRow(
	ctx context.Context,
	dataRowId *importfile.DataRowId,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"rows._id": dataRowId},
		bson.M{"$set": bson.M{"rows.$.state": importfile.Verified}},
	)

	return err
}

func (repository *ReadModelRepository) InvalidateDataRow(
	ctx context.Context,
	dataRowId *importfile.DataRowId,
) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"rows._id": dataRowId},
		bson.M{"$set": bson.M{"rows.$.state": importfile.Invalid}},
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

func (repository *ReadModelRepository) GetById(ctx context.Context, importFileId *importfile.Id) (*Entity, error) {
	var entity *Entity

	err := repository.collection().FindOne(ctx, bson.M{"_id": importFileId}).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) GetFileRowsById(
	ctx context.Context,
	importFileId *importfile.Id,
) (*FileRowsEntity, error) {
	var entity *FileRowsEntity

	findOneOptions := options.FindOne().SetSort(bson.M{"rows.date": 1})

	err := repository.collection().FindOne(ctx, bson.M{"_id": importFileId}, findOneOptions).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) GetFileRowByRowId(
	ctx context.Context,
	FileDataRowId *importfile.DataRowId,
) (*FileRowEntity, error) {
	var resultStruct struct {
		Rows []*FileRowEntity `bson:"rows"`
	}

	findOneOptions := &options.FindOneOptions{
		Projection: bson.M{"rows.$": 1},
	}

	err := repository.collection().
		FindOne(ctx, bson.M{"rows._id": uuid.MustParse(FileDataRowId.String())}, findOneOptions).
		Decode(&resultStruct)
	if err != nil {
		return nil, err
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}

	return resultStruct.Rows[0], nil
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

func (repository *ReadModelRepository) GetAndLockNextFileToParse(ctx context.Context) (*Entity, error) {
	var entity Entity

	err := repository.collection().
		FindOneAndUpdate(
			ctx,
			bson.M{
				"$and": bson.A{
					bson.M{
						"$or": bson.A{
							bson.M{"state": importfile.Imported},
							bson.M{"state": importfile.ParsingStarted},
							bson.M{"state": importfile.ParsingRestarted},
						},
					},
					bson.M{
						"$or": bson.A{
							bson.M{"locked_until": bson.M{"$lte": time.Now()}},
							bson.M{"locked_until": bson.M{"$exists": false}},
						},
					},
				},
			},
			bson.M{"$set": bson.M{"locked_until": time.Now().Add(5 * time.Minute)}},
			options.FindOneAndUpdate().SetSort(bson.D{{"import_date", 1}}),
		).
		Decode(&entity)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(importfile.AggregateType.String())
}

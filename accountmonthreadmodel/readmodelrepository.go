package accountmonthreadmodel

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sort"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountreadmodel"
	"walletaccountant/common"
	"walletaccountant/mongodb"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	StartMonth(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
		accountId *account.Id,
		startBalance float32,
		month time.Month,
		year uint,
	) error
	EndMonth(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
	) error
	RegisterAccountMovement(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
		eventData *accountmonth.NewAccountMovementRegisteredData,
	) error
}

type ReadModelReader interface {
	GetByAccountMonthId(ctx context.Context, accountMonthId *accountmonth.Id) (*Entity, error)
	GetByAccountActiveMonth(ctx context.Context, account *accountreadmodel.Entity) (*Entity, error)
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

func (repository *ReadModelRepository) StartMonth(
	ctx context.Context,
	accountMonthId *accountmonth.Id,
	accountId *account.Id,
	startBalance float32,
	month time.Month,
	year uint,
) error {
	newAccountMonth := Entity{
		AccountMonthId: accountMonthId,
		AccountId:      accountId,
		ActiveMonth: &EntityActiveMonth{
			Month: month,
			Year:  year,
		},
		Movements:      []*EntityMovement{},
		Balance:        startBalance,
		InitialBalance: startBalance,
		MonthEnded:     false,
	}

	_, err := repository.collection().InsertOne(ctx, newAccountMonth)

	return err

}

func (repository *ReadModelRepository) EndMonth(ctx context.Context, accountMonthId *accountmonth.Id) error {
	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": accountMonthId},
		bson.M{
			"$set": bson.M{"month_ended": true},
		},
	)

	return err

}

func (repository *ReadModelRepository) RegisterAccountMovement(
	ctx context.Context,
	accountMonthId *accountmonth.Id,
	eventData *accountmonth.NewAccountMovementRegisteredData,
) error {
	newMovementTypeEntity := EntityMovement{
		MovementTypeId:  eventData.MovementTypeId,
		Action:          eventData.Action,
		Amount:          eventData.Amount,
		Date:            eventData.Date,
		SourceAccountId: eventData.SourceAccountId,
		Description:     eventData.Description,
		Notes:           eventData.Notes,
		TagIds:          eventData.TagIds,
	}

	balanceChange := eventData.Amount
	if eventData.Action == common.Debit {
		balanceChange = eventData.Amount * -1
	}

	_, err := repository.collection().UpdateOne(
		ctx,
		bson.M{"_id": accountMonthId},
		bson.M{
			"$inc":  bson.M{"balance": balanceChange},
			"$push": bson.M{"movements": newMovementTypeEntity},
		},
	)

	return err
}

func (repository *ReadModelRepository) GetByAccountMonthId(ctx context.Context, accountMonthId *accountmonth.Id) (*Entity, error) {
	var entity *Entity

	err := repository.collection().
		FindOne(
			ctx,
			bson.M{"_id": accountMonthId},
			&options.FindOneOptions{Sort: bson.D{{"movements.date", 1}}},
		).
		Decode(&entity)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(
		entity.Movements,
		func(i, j int) bool { return entity.Movements[i].Date.Before(entity.Movements[j].Date) },
	)

	return entity, nil
}

func (repository *ReadModelRepository) GetByAccountActiveMonth(
	ctx context.Context,
	account *accountreadmodel.Entity,
) (*Entity, error) {
	accountMonthId, err := accountmonth.GenerateAccountMonthId(
		account.AccountId,
		account.ActiveMonth.Month,
		account.ActiveMonth.Year,
	)
	if err != nil {
		return nil, err
	}

	return repository.GetByAccountMonthId(ctx, accountMonthId)
}

func (repository *ReadModelRepository) collection() *mongo.Collection {
	return repository.client.Collection(accountmonth.AggregateType.String())
}

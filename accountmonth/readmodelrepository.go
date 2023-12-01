package accountmonth

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
	"walletaccountant/account"
	"walletaccountant/mongodb"
	"walletaccountant/movementtype"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelWriter interface {
	StartMonth(
		ctx context.Context,
		accountMonthId *Id,
		accountId *account.Id,
		startBalance float64,
		month time.Month,
		year uint,
	) error
	EndMonth(
		ctx context.Context,
		accountMonthId *Id,
	) error
	RegisterAccountMovement(
		ctx context.Context,
		accountMonthId *Id,
		movementTypeId *movementtype.Id,
		movementTypeType movementtype.Type,
		amount float64,
		date time.Time,
	) error
}

type ReadModelReader interface {
	GetByAccountMonthId(ctx context.Context, accountMonthId *Id) (*Entity, error)
	GetByAccountActiveMonth(ctx context.Context, account *account.Entity) (*Entity, error)
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
	accountMonthId *Id,
	accountId *account.Id,
	startBalance float64,
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
		Movements:  []*EntityMovement{},
		Balance:    startBalance,
		MonthEnded: false,
	}

	_, err := repository.collection().InsertOne(ctx, newAccountMonth)

	return err

}

func (repository *ReadModelRepository) EndMonth(ctx context.Context, accountMonthId *Id) error {
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
	accountMonthId *Id,
	movementTypeId *movementtype.Id,
	movementTypeType movementtype.Type,
	amount float64,
	date time.Time,
) error {
	newMovementTypeEntity := EntityMovement{
		MovementTypeId: movementTypeId,
		Amount:         amount,
		Date:           date,
	}

	balanceChange := amount
	if movementTypeType == movementtype.Debit {
		balanceChange = amount * -1
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

func (repository *ReadModelRepository) GetByAccountMonthId(ctx context.Context, accountMonthId *Id) (*Entity, error) {
	var entity *Entity

	err := repository.collection().FindOne(ctx, bson.M{"_id": accountMonthId}).Decode(&entity)
	if err != nil {
		return nil, err
	}

	return entity, nil
}

func (repository *ReadModelRepository) GetByAccountActiveMonth(
	ctx context.Context,
	account *account.Entity,
) (*Entity, error) {
	accountMonthId, err := GenerateAccountMonthId(
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
	return repository.client.Collection(AggregateType.String())
}

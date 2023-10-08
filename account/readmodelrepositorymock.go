package account

import (
	"context"
	"github.com/looplab/eventhorizon/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

var _ ReadModeler = &ReadModelRepositoryMock{}

type ReadModelRepositoryMock struct {
	CreateFn            func(ctx context.Context, account Entity) error
	UpdateActiveMonthFn func(ctx context.Context, accountId *Id, activeMonth EntityActiveMonth) error
	GetAllFn            func(ctx context.Context) ([]*Entity, error)
	GetByAccountIdFn    func(ctx context.Context, accountId *Id) (*Entity, error)
	GetByNameFn         func(ctx context.Context, name string) (*Entity, error)
}

func (repoMock *ReadModelRepositoryMock) Create(ctx context.Context, account Entity) error {
	if repoMock != nil && repoMock.CreateFn != nil {
		return repoMock.CreateFn(ctx, account)
	}

	return nil
}

func (repoMock *ReadModelRepositoryMock) UpdateActiveMonth(
	ctx context.Context,
	accountId *Id,
	activeMonth EntityActiveMonth,
) error {
	if repoMock != nil && repoMock.UpdateActiveMonthFn != nil {
		return repoMock.UpdateActiveMonthFn(ctx, accountId, activeMonth)
	}

	return nil
}

func (repoMock *ReadModelRepositoryMock) GetAll(ctx context.Context) ([]*Entity, error) {
	if repoMock != nil && repoMock.GetAllFn != nil {
		return repoMock.GetAllFn(ctx)
	}

	accountId1 := Id(uuid.MustParse("83528ee4-3f0f-43ea-a383-e3846c00fa38"))
	accountId2 := Id(uuid.MustParse("83528ee4-3f0f-43ea-a383-e3846c00fa40"))

	return []*Entity{
		repoMock.entity(
			&accountId1,
			"some bank name",
			"some name",
			Checking,
			1069,
			time.Date(2023, 9, 10, 0, 0, 0, 0, time.UTC),
			EUR,
			"my some notes",
			ActiveMonth{
				month: 9,
				year:  2023,
			},
		),
		repoMock.entity(
			&accountId2,
			"another bank name",
			"another name",
			Savings,
			1169,
			time.Date(2022, 8, 10, 0, 0, 0, 0, time.UTC),
			USD,
			"my another notes",
			ActiveMonth{
				month: 8,
				year:  2023,
			},
		),
	}, nil
}

func (repoMock *ReadModelRepositoryMock) GetByAccountId(ctx context.Context, accountId *Id) (*Entity, error) {
	if repoMock != nil && repoMock.GetByAccountIdFn != nil {
		return repoMock.GetByAccountIdFn(ctx, accountId)
	}

	return repoMock.entity(
		accountId,
		"bank name",
		"name",
		Checking,
		1069,
		time.Date(2023, 9, 10, 0, 0, 0, 0, time.UTC),
		EUR,
		"my notes",
		ActiveMonth{
			month: 9,
			year:  2023,
		},
	), nil
}

func (repoMock *ReadModelRepositoryMock) GetByName(ctx context.Context, name string) (*Entity, error) {
	if repoMock != nil && repoMock.GetByNameFn != nil {
		return repoMock.GetByNameFn(ctx, name)
	}

	return nil, mongo.ErrNoDocuments
}

func (repoMock *ReadModelRepositoryMock) entity(
	accountId *Id,
	bankName string,
	name string,
	accountType Type,
	startingBalance float64,
	startingBalanceDate time.Time,
	currency Currency,
	notes string,
	activeMonth ActiveMonth,
) *Entity {
	return &Entity{
		AccountId:           accountId,
		BankName:            bankName,
		Name:                name,
		AccountType:         accountType,
		StartingBalance:     startingBalance,
		StartingBalanceDate: startingBalanceDate,
		Currency:            currency,
		Notes:               notes,
		ActiveMonth: EntityActiveMonth{
			Month: activeMonth.Month(),
			Year:  activeMonth.Year(),
		},
	}
}

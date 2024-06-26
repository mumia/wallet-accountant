package accountreadmodel

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
	"walletaccountant/account"
	"walletaccountant/common"
)

var _ ReadModeler = &ReadModelRepositoryMock{}

type ReadModelRepositoryMock struct {
	CreateFn            func(ctx context.Context, account Entity) error
	UpdateActiveMonthFn func(ctx context.Context, accountId *account.Id, activeMonth EntityActiveMonth) error
	GetAllFn            func(ctx context.Context) ([]*Entity, error)
	GetByAccountIdFn    func(ctx context.Context, accountId *account.Id) (*Entity, error)
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
	accountId *account.Id,
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

	accountId1 := account.IdFromUUIDString("83528ee4-3f0f-43ea-a383-e3846c00fa38")
	accountId2 := account.IdFromUUIDString("83528ee4-3f0f-43ea-a383-e3846c00fa40")

	notes := "my some notes"
	notes1 := "my another notes"

	return []*Entity{
		repoMock.entity(
			accountId1,
			"some bank name",
			"some name",
			common.Checking,
			106900,
			time.Date(2023, 9, 10, 0, 0, 0, 0, time.UTC),
			account.EUR,
			&notes,
			account.NewActiveMonth(9, 2023),
		),
		repoMock.entity(
			accountId2,
			"another bank name",
			"another name",
			common.Savings,
			116900,
			time.Date(2022, 8, 10, 0, 0, 0, 0, time.UTC),
			account.USD,
			&notes1,
			account.NewActiveMonth(8, 2023),
		),
	}, nil
}

func (repoMock *ReadModelRepositoryMock) GetByAccountId(ctx context.Context, accountId *account.Id) (*Entity, error) {
	if repoMock != nil && repoMock.GetByAccountIdFn != nil {
		return repoMock.GetByAccountIdFn(ctx, accountId)
	}

	notes := "my notes"
	return repoMock.entity(
		accountId,
		"bank name",
		"name",
		common.Checking,
		106900,
		time.Date(2023, 9, 10, 0, 0, 0, 0, time.UTC),
		account.EUR,
		&notes,
		account.NewActiveMonth(9, 2023),
	), nil
}

func (repoMock *ReadModelRepositoryMock) GetByName(ctx context.Context, name string) (*Entity, error) {
	if repoMock != nil && repoMock.GetByNameFn != nil {
		return repoMock.GetByNameFn(ctx, name)
	}

	return nil, mongo.ErrNoDocuments
}

func (repoMock *ReadModelRepositoryMock) entity(
	accountId *account.Id,
	bankName account.BankName,
	name string,
	accountType common.AccountType,
	startingBalance int64,
	startingBalanceDate time.Time,
	currency account.Currency,
	notes *string,
	activeMonth account.ActiveMonth,
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

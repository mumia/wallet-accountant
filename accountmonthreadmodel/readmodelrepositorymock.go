package accountmonthreadmodel

import (
	"context"
	"time"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountreadmodel"
)

var _ ReadModeler = &ReadModelRepositoryMock{}

type ReadModelRepositoryMock struct {
	StartMonthFn func(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
		accountId *account.Id,
		startBalance int64,
		month time.Month,
		year uint,
	) error
	EndMonthFn func(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
	) error
	RegisterAccountMovementFn func(
		ctx context.Context,
		accountMonthId *accountmonth.Id,
		eventData *accountmonth.NewAccountMovementRegisteredData,
	) error
	GetByAccountMonthIdFn     func(ctx context.Context, accountMonthId *accountmonth.Id) (*Entity, error)
	GetByAccountActiveMonthFn func(ctx context.Context, account *accountreadmodel.Entity) (*Entity, error)
}

func (mock *ReadModelRepositoryMock) StartMonth(
	ctx context.Context,
	accountMonthId *accountmonth.Id,
	accountId *account.Id,
	startBalance int64,
	month time.Month,
	year uint,
) error {
	if mock != nil && mock.StartMonthFn != nil {
		return mock.StartMonthFn(ctx, accountMonthId, accountId, startBalance, month, year)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) EndMonth(
	ctx context.Context,
	accountMonthId *accountmonth.Id,
) error {
	if mock != nil && mock.EndMonthFn != nil {
		return mock.EndMonthFn(ctx, accountMonthId)
	}

	return nil
}
func (mock *ReadModelRepositoryMock) RegisterAccountMovement(
	ctx context.Context,
	accountMonthId *accountmonth.Id,
	eventData *accountmonth.NewAccountMovementRegisteredData,
) error {
	if mock != nil && mock.RegisterAccountMovementFn != nil {
		return mock.RegisterAccountMovementFn(ctx, accountMonthId, eventData)
	}

	return nil
}
func (mock *ReadModelRepositoryMock) GetByAccountMonthId(ctx context.Context, accountMonthId *accountmonth.Id) (*Entity, error) {
	if mock != nil && mock.GetByAccountMonthIdFn != nil {
		return mock.GetByAccountMonthIdFn(ctx, accountMonthId)
	}

	return nil, nil
}
func (mock *ReadModelRepositoryMock) GetByAccountActiveMonth(ctx context.Context, account *accountreadmodel.Entity) (*Entity, error) {
	if mock != nil && mock.GetByAccountActiveMonthFn != nil {
		return mock.GetByAccountActiveMonthFn(ctx, account)
	}

	return nil, nil
}

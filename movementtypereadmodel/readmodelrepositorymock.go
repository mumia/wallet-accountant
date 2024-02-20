package movementtypereadmodel

import (
	"context"
	"walletaccountant/account"
	"walletaccountant/movementtype"
)

var _ ReadModeler = &ReadModelRepository{}

type ReadModelRepositoryMock struct {
	CreateFn              func(ctx context.Context, movementType *Entity) error
	GetAllFn              func(ctx context.Context) ([]*Entity, error)
	GetByMovementTypeIdFn func(ctx context.Context, movementTypeId *movementtype.Id) (*Entity, error)
	GetByAccountIdFn      func(ctx context.Context, accountId *account.Id) ([]*Entity, error)
}

func (mock *ReadModelRepositoryMock) Create(ctx context.Context, movementType *Entity) error {
	if mock != nil && mock.CreateFn != nil {
		return mock.CreateFn(ctx, movementType)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) GetAll(ctx context.Context) ([]*Entity, error) {
	if mock != nil && mock.GetAllFn != nil {
		return mock.GetAllFn(ctx)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetByMovementTypeId(ctx context.Context, movementTypeId *movementtype.Id) (*Entity, error) {
	if mock != nil && mock.GetByMovementTypeIdFn != nil {
		return mock.GetByMovementTypeIdFn(ctx, movementTypeId)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetByAccountId(ctx context.Context, accountId *account.Id) ([]*Entity, error) {
	if mock != nil && mock.GetByAccountIdFn != nil {
		return mock.GetByAccountIdFn(ctx, accountId)
	}

	return nil, nil
}

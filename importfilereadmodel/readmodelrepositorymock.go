package importfilereadmodel

import (
	"context"
	"time"
	"walletaccountant/account"
	"walletaccountant/importfile"
)

var _ ReadModeler = &ReadModelRepositoryMock{}

type ReadModelRepositoryMock struct {
	RegisterFn                  func(ctx context.Context, importFile Entity) error
	StartParseFn                func(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	RestartParseFn              func(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	EndParseFn                  func(ctx context.Context, importFileId *importfile.Id, date time.Time) error
	FailParseFn                 func(ctx context.Context, importFileId *importfile.Id, date time.Time, code string, reason string) error
	AddFileDataRowFn            func(ctx context.Context, importFileId *importfile.Id, dataRow FileRowEntity) error
	VerifyDataRowFn             func(ctx context.Context, dataRowId *importfile.DataRowId) error
	InvalidateDataRowFn         func(ctx context.Context, dataRowId *importfile.DataRowId) error
	GetAllFn                    func(ctx context.Context) ([]*Entity, error)
	GetByIdFn                   func(ctx context.Context, importFileId *importfile.Id) (*Entity, error)
	GetFileRowsByIdFn           func(ctx context.Context, importFileId *importfile.Id) (*FileRowsEntity, error)
	GetFileRowByRowIdFn         func(ctx context.Context, FileDataRowId *importfile.DataRowId) (*FileRowEntity, error)
	GetByAccountIdFn            func(ctx context.Context, accountId *account.Id) ([]*Entity, error)
	GetAndLockNextFileToParseFn func(ctx context.Context) (*Entity, error)
}

func (mock *ReadModelRepositoryMock) Register(ctx context.Context, importFile Entity) error {
	if mock != nil && mock.RegisterFn != nil {
		return mock.RegisterFn(ctx, importFile)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) StartParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error {
	if mock != nil && mock.StartParseFn != nil {
		return mock.StartParseFn(ctx, importFileId, date)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) RestartParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error {
	if mock != nil && mock.RestartParseFn != nil {
		return mock.RestartParseFn(ctx, importFileId, date)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) EndParse(ctx context.Context, importFileId *importfile.Id, date time.Time) error {
	if mock != nil && mock.EndParseFn != nil {
		return mock.EndParseFn(ctx, importFileId, date)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) FailParse(ctx context.Context, importFileId *importfile.Id, date time.Time, code string, reason string) error {
	if mock != nil && mock.FailParseFn != nil {
		return mock.FailParseFn(ctx, importFileId, date, code, reason)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) AddFileDataRow(ctx context.Context, importFileId *importfile.Id, dataRow FileRowEntity) error {
	if mock != nil && mock.AddFileDataRowFn != nil {
		return mock.AddFileDataRowFn(ctx, importFileId, dataRow)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) VerifyDataRow(ctx context.Context, dataRowId *importfile.DataRowId) error {
	if mock != nil && mock.VerifyDataRowFn != nil {
		return mock.VerifyDataRowFn(ctx, dataRowId)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) InvalidateDataRow(ctx context.Context, dataRowId *importfile.DataRowId) error {
	if mock != nil && mock.InvalidateDataRowFn != nil {
		return mock.InvalidateDataRowFn(ctx, dataRowId)
	}

	return nil
}

func (mock *ReadModelRepositoryMock) GetAll(ctx context.Context) ([]*Entity, error) {
	if mock != nil && mock.GetAllFn != nil {
		return mock.GetAllFn(ctx)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetById(ctx context.Context, importFileId *importfile.Id) (*Entity, error) {
	if mock != nil && mock.GetByIdFn != nil {
		return mock.GetByIdFn(ctx, importFileId)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetFileRowsById(ctx context.Context, importFileId *importfile.Id) (*FileRowsEntity, error) {
	if mock != nil && mock.GetFileRowsByIdFn != nil {
		return mock.GetFileRowsByIdFn(ctx, importFileId)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetFileRowByRowId(ctx context.Context, FileDataRowId *importfile.DataRowId) (*FileRowEntity, error) {
	if mock != nil && mock.GetFileRowByRowIdFn != nil {
		return mock.GetFileRowByRowIdFn(ctx, FileDataRowId)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetByAccountId(ctx context.Context, accountId *account.Id) ([]*Entity, error) {
	if mock != nil && mock.GetByAccountIdFn != nil {
		return mock.GetByAccountIdFn(ctx, accountId)
	}

	return nil, nil
}

func (mock *ReadModelRepositoryMock) GetAndLockNextFileToParse(ctx context.Context) (*Entity, error) {
	if mock != nil && mock.GetAndLockNextFileToParseFn != nil {
		return mock.GetAndLockNextFileToParseFn(ctx)
	}

	return nil, nil
}

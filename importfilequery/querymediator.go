package importfilequery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/definitions"
	"walletaccountant/importfile"
	"walletaccountant/importfilereadmodel"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	ImportFile(
		ctx *gin.Context,
		importFileId *importfile.Id,
	) (*importfilereadmodel.Entity, *definitions.WalletAccountantError)
	ImportFileRows(
		ctx *gin.Context,
		importFileId *importfile.Id,
	) (*importfilereadmodel.FileRowsEntity, *definitions.WalletAccountantError)
	ImportFiles(ctx *gin.Context) ([]*importfilereadmodel.Entity, *definitions.WalletAccountantError)
	ImportFilesByAccount(
		ctx *gin.Context,
		accountId *account.Id,
	) ([]*importfilereadmodel.Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository importfilereadmodel.ReadModeler
}

func NewQueryMediator(repository importfilereadmodel.ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository}
}

func (mediator QueryMediator) ImportFile(ctx *gin.Context, importFileId *importfile.Id) (*importfilereadmodel.Entity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetById(ctx, importFileId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entity, nil
}

func (mediator QueryMediator) ImportFiles(ctx *gin.Context) ([]*importfilereadmodel.Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetAll(ctx)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

func (mediator QueryMediator) ImportFileRows(
	ctx *gin.Context,
	importFileId *importfile.Id,
) (*importfilereadmodel.FileRowsEntity, *definitions.WalletAccountantError) {
	entity, err := mediator.repository.GetFileRowsById(ctx, importFileId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entity, nil
}

func (mediator QueryMediator) ImportFilesByAccount(ctx *gin.Context, accountId *account.Id) ([]*importfilereadmodel.Entity, *definitions.WalletAccountantError) {
	entities, err := mediator.repository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, definitions.GenericError(err, nil)
	}

	return entities, nil
}

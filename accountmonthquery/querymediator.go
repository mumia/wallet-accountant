package accountmonthquery

import (
	"github.com/gin-gonic/gin"
	"walletaccountant/account"
	"walletaccountant/accountmonth"
	"walletaccountant/accountmonthreadmodel"
	"walletaccountant/accountreadmodel"
	"walletaccountant/definitions"
)

var _ QueryMediatorer = &QueryMediator{}

type QueryMediatorer interface {
	AccountMonth(ctx *gin.Context, accountId *accountmonth.Id) (*accountmonthreadmodel.Entity, *definitions.WalletAccountantError)
}

type QueryMediator struct {
	repository        accountmonthreadmodel.ReadModeler
	accountRepository accountreadmodel.ReadModeler
}

func NewQueryMediator(repository accountmonthreadmodel.ReadModeler, accountRepository accountreadmodel.ReadModeler) *QueryMediator {
	return &QueryMediator{repository: repository, accountRepository: accountRepository}
}

func (mediator QueryMediator) AccountMonth(
	ctx *gin.Context,
	accountId *account.Id,
) (*accountmonthreadmodel.Entity, *definitions.WalletAccountantError) {
	accountEntity, err := mediator.accountRepository.GetByAccountId(ctx, accountId)
	if err != nil {
		return nil, definitions.GenericError(err, definitions.ErrorContext{"accountId": accountId.String()})
	}

	if accountEntity == nil {
		return nil, accountmonth.NonExistentAccountIdError(accountId.String())
	}

	entity, err := mediator.repository.GetByAccountActiveMonth(ctx, accountEntity)
	if err != nil {
		return nil, definitions.GenericError(
			err,
			definitions.ErrorContext{
				"accountId": accountId.String(),
				"month":     accountEntity.ActiveMonth.Month,
				"year":      accountEntity.ActiveMonth.Year,
			},
		)
	}

	if entity == nil {
		return nil, accountmonth.NonExistentAccountMonthError(
			accountEntity.AccountId.String(),
			"",
			int(accountEntity.ActiveMonth.Month),
			int(accountEntity.ActiveMonth.Year),
		)
	}

	return entity, nil
}

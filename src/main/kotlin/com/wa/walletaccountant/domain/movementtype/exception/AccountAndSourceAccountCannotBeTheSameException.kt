package com.wa.walletaccountant.domain.movementtype.exception

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.exception.AggregateLogicException

class AccountAndSourceAccountCannotBeTheSameException(
    accountId: AccountId,
    sourceAccountId: AccountId
) : AggregateLogicException(
    "MovementType",
    "Account and Source account cannot be the same. [accountId: %s] [sourceAccountId: %s]".format(
        accountId,
        sourceAccountId
    )
)
package com.wa.walletaccountant.application.projection.account

import com.wa.walletaccountant.domain.account.account.AccountId
import java.time.Month
import java.time.Year

class CurrentMonthUpdateFailureException(val accountId: AccountId, val month: Month, val year: Year) : RuntimeException(
    "Failed updating current month on account. [AccountId=%s][Month=%d][Year=%d]".format(
        accountId,
        month,
        year
    )
)
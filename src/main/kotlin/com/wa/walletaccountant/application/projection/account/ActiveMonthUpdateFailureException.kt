package com.wa.walletaccountant.application.projection.account

import com.wa.walletaccountant.domain.account.account.AccountId
import java.time.Month
import java.time.Year

class ActiveMonthUpdateFailureException(val accountId: AccountId, val month: Month, val year: Year) : RuntimeException(
    "Failed updating active month on account. [AccountId=%s][Month=%d][Year=%d]".format(
        accountId,
        month.value,
        year.value
    )
)
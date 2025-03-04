package com.wa.walletaccountant.domain.account.event

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import java.time.Month
import java.time.Year

data class NextMonthStartedEvent(
    val accountId: AccountId,
    val balance: Money,
    val month: Month,
    val year: Year,
)

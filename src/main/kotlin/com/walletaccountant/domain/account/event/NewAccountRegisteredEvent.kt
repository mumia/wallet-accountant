package com.walletaccountant.domain.account.event

import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.axonframework.eventsourcing.annotation.EventTag
import java.time.Month
import java.time.Year

data class NewAccountRegisteredEvent(
    @EventTag(key = "accountId") val accountId: AccountId,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val currency: Currency,
    val startingDate: Date,
    val month: Month,
    val year: Year,
    val notes: String? = null
)

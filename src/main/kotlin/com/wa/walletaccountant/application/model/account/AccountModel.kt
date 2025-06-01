package com.wa.walletaccountant.application.model.account

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.common.Currency
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import io.swagger.v3.oas.annotations.media.Schema
import java.time.Month
import java.time.Year

data class AccountModel(
    val aggregateId: String,
    val accountId: AccountId,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val startingBalanceDate: Date,
    val currency: Currency,
    val notes: String?,
    val activeMonth: ActiveMonth,
) {
    data class ActiveMonth(
        val month: Month,
        @Schema(type = "integer", format = "int32")
        val year: Year,
    )
}

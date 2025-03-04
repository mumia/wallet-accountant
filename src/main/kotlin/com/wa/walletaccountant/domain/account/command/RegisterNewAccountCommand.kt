package com.wa.walletaccountant.domain.account.command

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.common.Currency
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class RegisterNewAccountCommand(
    @TargetAggregateIdentifier
    val accountId: AccountId,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val startingBalanceDate: Date,
    val currency: Currency,
    val notes: String?,
)

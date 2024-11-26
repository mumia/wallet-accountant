package com.wa.walletaccountant.adapter.`in`.web.account.mapper

import com.wa.walletaccountant.adapter.`in`.web.account.request.NewAccountRequest
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.common.Currency
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money

class AccountMapper private constructor() {
    companion object {
        fun toCommand(
            accountId: AccountId,
            request: NewAccountRequest,
        ): RegisterNewAccountCommand {
            val currency = Currency.valueOf(request.currency)

            return RegisterNewAccountCommand(
                accountId,
                BankName.valueOf(request.bankName),
                request.name,
                AccountType.valueOf(request.accountType),
                Money(request.startingBalance, currency),
                Date.fromString(request.startingBalanceDate),
                currency,
                request.notes,
            )
        }
    }
}

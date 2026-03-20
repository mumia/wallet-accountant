package com.walletaccountant.domain.account.command

import com.walletaccountant.application.interceptor.HasAggregateId
import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.axonframework.modelling.annotation.TargetEntityId
import kotlin.uuid.Uuid

data class RegisterNewAccountCommand(
    @TargetEntityId val accountId: AccountId,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val currency: Currency,
    val startingDate: Date,
    val notes: String? = null
) : HasAggregateId<RegisterNewAccountCommand> {

    override fun withNewId(): RegisterNewAccountCommand =
        copy(accountId = AccountId(Uuid.random()))
}

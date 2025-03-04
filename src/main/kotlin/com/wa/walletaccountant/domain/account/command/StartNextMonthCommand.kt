package com.wa.walletaccountant.domain.account.command

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class StartNextMonthCommand(
    @TargetAggregateIdentifier
    val accountId: AccountId,
    val balance: Money,
)

package com.wa.walletaccountant.domain.ledger.command

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class StartMonthCommand(
    @TargetAggregateIdentifier
    val ledgerId: LedgerId,
    val startBalance: Money,
)

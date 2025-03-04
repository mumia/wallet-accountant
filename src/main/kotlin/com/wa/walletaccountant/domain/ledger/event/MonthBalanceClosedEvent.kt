package com.wa.walletaccountant.domain.ledger.event

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId

data class MonthBalanceClosedEvent(
    val ledgerId: LedgerId,
    val closeBalance: Money,
)

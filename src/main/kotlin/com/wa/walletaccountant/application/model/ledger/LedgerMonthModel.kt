package com.wa.walletaccountant.application.model.ledger

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId

data class LedgerMonthModel(
    val ledgerId: LedgerId,
    val balance: Money,
    val transactions: Set<LedgerTransactionModel>,
    val closed: Boolean = false,
)

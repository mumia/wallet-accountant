package com.wa.walletaccountant.application.model.ledger

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import java.time.Month
import java.time.Year

data class LedgerMonthModel(
    val ledgerId: LedgerId,
    val accountId: AccountId,
    val month: Month,
    val year: Year,
    val initialBalance: Money,
    val balance: Money,
    val transactions: Set<LedgerTransactionModel>,
    val closed: Boolean = false,
)

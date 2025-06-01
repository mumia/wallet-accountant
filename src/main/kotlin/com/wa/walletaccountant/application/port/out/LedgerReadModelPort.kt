package com.wa.walletaccountant.application.port.out

import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import java.util.Optional

interface LedgerReadModelPort {
    fun openMonthBalance(id: LedgerId, initialBalance: Money)

    fun closeMonthBalance(id: LedgerId, balance: Money): Boolean

    fun registerTransaction(id: LedgerId, transactionModel: LedgerTransactionModel): Boolean

    fun readCurrentMonthLedger(accountId: AccountId): Optional<LedgerMonthModel>

    fun readLedgerMonth(id: LedgerId): Optional<LedgerMonthModel>
}

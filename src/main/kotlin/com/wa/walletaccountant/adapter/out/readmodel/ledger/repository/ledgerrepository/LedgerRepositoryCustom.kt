package com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.ledgerrepository

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerTransactionDocument
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId

interface LedgerRepositoryCustom {
    fun endMonth(id: LedgerId, balance: Money): Boolean

    fun registerTransaction(id: LedgerId, transaction: LedgerTransactionDocument): Boolean
}

package com.wa.walletaccountant.adapter.out.readmodel.ledger.document

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.data.annotation.TypeAlias
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.data.mongodb.core.mapping.MongoId

@Document
@TypeAlias("LedgerMonth")
data class LedgerMonthDocument(
    @MongoId
    val ledgerId: LedgerId,
    val balance: Money,
    val transactions: Set<LedgerTransactionDocument>,
    val closed: Boolean = false,
)

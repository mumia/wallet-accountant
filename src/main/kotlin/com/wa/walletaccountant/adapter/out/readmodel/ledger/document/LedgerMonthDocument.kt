package com.wa.walletaccountant.adapter.out.readmodel.ledger.document

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.data.annotation.TypeAlias
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.data.mongodb.core.mapping.MongoId
import java.time.Month
import java.time.Year

@Document
@TypeAlias("LedgerMonth")
data class LedgerMonthDocument(
    @MongoId
    val ledgerId: LedgerId,
    val accountId: AccountId,
    val month: Month,
    val year: Year,
    val initialBalance: Money,
    val transactions: Set<LedgerTransactionDocument>,
    val closed: Boolean = false,
)

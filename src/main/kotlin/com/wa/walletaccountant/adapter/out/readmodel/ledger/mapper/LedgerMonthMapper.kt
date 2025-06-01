package com.wa.walletaccountant.adapter.out.readmodel.ledger.mapper

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.stereotype.Service
import java.util.stream.Collectors

@Service
object LedgerMonthMapper {
    fun toDocument(id: LedgerId, initialBalance: Money): LedgerMonthDocument =
        LedgerMonthDocument(
            ledgerId = id,
            accountId = id.accountId,
            month = id.month,
            year = id.year,
            initialBalance = initialBalance,
            transactions = emptySet(),
            closed = false,
        )

    fun toDocument(model: LedgerMonthModel): LedgerMonthDocument =
        LedgerMonthDocument(
            ledgerId = model.ledgerId,
            accountId = model.accountId,
            month = model.month,
            year = model.year,
            initialBalance = model.initialBalance,
            transactions = model.transactions
                .stream()
                .map { LedgerTransactionMapper.toDocument(it) }
                .collect(Collectors.toSet()),
            closed = model.closed
        )

    fun toModel(document: LedgerMonthDocument): LedgerMonthModel {
        var balance = document.initialBalance
        val transactions = LinkedHashSet(
            document.transactions
                .map {
                    balance = balance.add(it.amount)

                    LedgerTransactionMapper.toModel(it)
                }
                .toSortedSet(
                    compareBy<LedgerTransactionModel> { it.date.timestamp() }.thenBy { it.transactionId.toString() }
                )
        )

        return LedgerMonthModel(
            ledgerId = document.ledgerId,
            accountId = document.accountId,
            month = document.month,
            year = document.year,
            initialBalance = document.initialBalance,
            balance = balance,
            transactions = transactions,
            closed = document.closed,
        )
    }
}
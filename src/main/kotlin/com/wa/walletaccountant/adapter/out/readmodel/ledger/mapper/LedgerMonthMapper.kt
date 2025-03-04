package com.wa.walletaccountant.adapter.out.readmodel.ledger.mapper

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.stereotype.Service
import java.util.stream.Collectors

@Service
object LedgerMonthMapper {
    fun toDocument(id: LedgerId, balance: Money): LedgerMonthDocument =
        LedgerMonthDocument(
            ledgerId = id,
            balance = balance,
            transactions = emptySet(),
            closed = false,
        )

    fun toDocument(model: LedgerMonthModel): LedgerMonthDocument =
        LedgerMonthDocument(
            ledgerId = model.ledgerId,
            balance = model.balance,
            transactions = model.transactions
                .stream()
                .map { LedgerTransactionMapper.toDocument(it) }
                .collect(Collectors.toSet()),
            closed = model.closed
        )

    fun toModel(document: LedgerMonthDocument): LedgerMonthModel =
        LedgerMonthModel(
            ledgerId = document.ledgerId,
            balance = document.balance,
            transactions = LinkedHashSet(
                document.transactions
                .stream()
                .map { LedgerTransactionMapper.toModel(it) }
                .collect(Collectors.toSet())
            ),
            closed = document.closed,
        )
}
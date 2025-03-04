package com.wa.walletaccountant.application.projection.ledger

import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.application.port.out.LedgerReadModelPort
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceClosedEvent
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceOpenedEvent
import com.wa.walletaccountant.domain.ledger.event.TransactionRegisteredEvent
import org.axonframework.config.ProcessingGroup
import org.axonframework.eventhandling.EventHandler
import org.springframework.stereotype.Component

@Component
@ProcessingGroup("ledger-read-model")
class LedgerReadModelProjection(private val readModelPort: LedgerReadModelPort) {
    @EventHandler
    fun on(event: MonthBalanceOpenedEvent) {
        readModelPort.openMonthBalance(event.ledgerId, event.startBalance)
    }

    @EventHandler
    fun on(event: MonthBalanceClosedEvent) {
        readModelPort.closeMonthBalance(event.ledgerId, event.closeBalance)
    }

    @EventHandler
    fun on(event: TransactionRegisteredEvent) {
        readModelPort.registerTransaction(
            event.ledgerId,
            LedgerTransactionModel(
                transactionId = event.transactionId,
                movementTypeId = event.movementTypeId,
                action = event.action,
                amount = event.amount,
                date = event.date,
                sourceAccountId = event.sourceAccountId,
                description = event.description,
                notes = event.notes,
                tagIds = event.tagIds,
            )
        )
    }
}

package com.wa.walletaccountant.domain.ledger

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.CloseBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.command.OpenBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.command.RegisterTransactionCommand
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceClosedEvent
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceOpenedEvent
import com.wa.walletaccountant.domain.ledger.event.TransactionRegisteredEvent
import com.wa.walletaccountant.domain.ledger.exception.CloseBalanceDoesNotMatchCurrentBalanceException
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Credit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import org.axonframework.commandhandling.CommandHandler
import org.axonframework.eventsourcing.EventSourcingHandler
import org.axonframework.extensions.kotlin.applyEvent
import org.axonframework.modelling.command.AggregateCreationPolicy
import org.axonframework.modelling.command.AggregateIdentifier
import org.axonframework.modelling.command.CreationPolicy
import org.axonframework.spring.stereotype.Aggregate

@Aggregate
class LedgerAggregrate {
    @AggregateIdentifier
    private lateinit var aggregateId: LedgerId
    private lateinit var startBalance: Money
    private lateinit var transactions: MutableMap<Long, Money>

    @CommandHandler
    @CreationPolicy(AggregateCreationPolicy.ALWAYS)
    fun on(command: OpenBalanceForMonthCommand) {
        applyEvent(
            MonthBalanceOpenedEvent(
                ledgerId = command.ledgerId,
                startBalance = command.startBalance
            )
        )
    }

    @CommandHandler
    fun on(command: RegisterTransactionCommand) {
        applyEvent(
            TransactionRegisteredEvent(
                ledgerId = command.ledgerId,
                transactionId = command.transactionId,
                movementTypeId = command.movementTypeId,
                action = if (command.amount.isNegative()) Debit else Credit,
                amount = command.amount,
                date = command.date,
                sourceAccountId = command.sourceAccountId,
                description = command.description,
                notes = command.notes,
                tagIds = command.tagIds,
            )
        )
    }

    @CommandHandler
    fun on(command: CloseBalanceForMonthCommand) {
        var currentBalance = startBalance
        for (value in transactions.values) {
            currentBalance = currentBalance.add(value)
        }

        if (currentBalance != command.endBalance) {
            throw CloseBalanceDoesNotMatchCurrentBalanceException(
                ledgerId = command.ledgerId,
                currentBalance = currentBalance,
                endBalance = command.endBalance,
            )
        }

        applyEvent(
            MonthBalanceClosedEvent(
                ledgerId = command.ledgerId,
                closeBalance = command.endBalance,
            )
        )
    }

    @EventSourcingHandler
    fun on(event: MonthBalanceOpenedEvent) {
        aggregateId = event.ledgerId
        startBalance = event.startBalance
        transactions = sortedMapOf()
    }

    @EventSourcingHandler
    fun on(event: TransactionRegisteredEvent) {
        transactions.put(event.date.timestamp(), event.amount)
    }
}
package com.wa.walletaccountant.domain.ledger

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.EndMonthCommand
import com.wa.walletaccountant.domain.ledger.command.RegisterLedgerMovementCommand
import com.wa.walletaccountant.domain.ledger.command.StartMonthCommand
import com.wa.walletaccountant.domain.ledger.event.LedgerMovementRegisteredEvent
import com.wa.walletaccountant.domain.ledger.event.MonthEndedEvent
import com.wa.walletaccountant.domain.ledger.event.MonthStartedEvent
import com.wa.walletaccountant.domain.ledger.exception.EndBalanceDoesNotMatchCurrentBalanceException
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
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
    private var aggregateId: LedgerId? = null
    private lateinit var currentBalance: Money

    @CommandHandler
    @CreationPolicy(AggregateCreationPolicy.ALWAYS)
    fun on(command: StartMonthCommand) {
        applyEvent(
            MonthStartedEvent(
                ledgerId = command.ledgerId,
                startBalance = command.startBalance
            )
        )
    }

    @CommandHandler
    fun on(command: RegisterLedgerMovementCommand) {
        applyEvent(
            LedgerMovementRegisteredEvent(
                ledgerId = command.ledgerId,
                movementId = command.movementId,
                movementTypeId = command.movementTypeId,
                action = command.action,
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
    fun on(command: EndMonthCommand) {
        if (currentBalance != command.endBalance) {
            throw EndBalanceDoesNotMatchCurrentBalanceException(
                ledgerId = command.ledgerId,
                currentBalance = currentBalance,
                endBalance = command.endBalance,
            )
        }

        applyEvent(
            MonthEndedEvent(
                ledgerId = command.ledgerId,
                endBalance = command.endBalance,
            )
        )
    }

    @EventSourcingHandler
    fun on(event: MonthStartedEvent) {
        aggregateId = event.ledgerId
        currentBalance = event.startBalance
    }

    @EventSourcingHandler
    fun on(event: LedgerMovementRegisteredEvent) {
        if (event.action == Debit) {
            currentBalance = currentBalance.subtract(event.amount)
        } else {
            currentBalance = currentBalance.add(event.amount)
        }
    }

//    @EventSourcingHandler
//    fun on(event: MonthEndedEvent) {
// TODO: mark month has ended?
//    }
}
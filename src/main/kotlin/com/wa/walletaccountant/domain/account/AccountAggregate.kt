package com.wa.walletaccountant.domain.account

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.account.command.StartNextMonthCommand
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.wa.walletaccountant.domain.account.event.NextMonthStartedEvent
import com.wa.walletaccountant.domain.common.Date
import org.axonframework.commandhandling.CommandHandler
import org.axonframework.eventsourcing.EventSourcingHandler
import org.axonframework.extensions.kotlin.applyEvent
import org.axonframework.modelling.command.AggregateCreationPolicy
import org.axonframework.modelling.command.AggregateIdentifier
import org.axonframework.modelling.command.CreationPolicy
import org.axonframework.spring.stereotype.Aggregate
import java.time.Month
import java.time.Year

@Aggregate
class AccountAggregate {
    @AggregateIdentifier
    private lateinit var aggregateId: AccountId
    private lateinit var month: Month
    private lateinit var year: Year

    @CommandHandler
    @CreationPolicy(AggregateCreationPolicy.ALWAYS)
    fun on(command: RegisterNewAccountCommand) {
        applyEvent(
            NewAccountRegisteredEvent(
                command.accountId,
                command.bankName,
                command.name,
                command.accountType,
                command.startingBalance,
                command.startingBalanceDate,
                command.currency,
                command.notes,
            ),
        )
    }

    @CommandHandler
    fun on (command: StartNextMonthCommand) {
        val nextMonthDate = Date.nextMonth(month, year)

        applyEvent(
            NextMonthStartedEvent(
                accountId = command.accountId,
                balance = command.balance,
                month = nextMonthDate.month(),
                year = nextMonthDate.year(),
            )
        )
    }

    @EventSourcingHandler
    fun on(event: NewAccountRegisteredEvent) {
        aggregateId = event.accountId
        month = event.startingBalanceDate.month()
        year = event.startingBalanceDate.year()
    }

    @EventSourcingHandler
    fun on(event: NextMonthStartedEvent) {
        month = event.month
        year = event.year
    }
}

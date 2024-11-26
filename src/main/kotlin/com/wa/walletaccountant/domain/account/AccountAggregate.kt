package com.wa.walletaccountant.domain.account

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import org.axonframework.commandhandling.CommandHandler
import org.axonframework.eventsourcing.EventSourcingHandler
import org.axonframework.extensions.kotlin.applyEvent
import org.axonframework.modelling.command.AggregateCreationPolicy
import org.axonframework.modelling.command.AggregateIdentifier
import org.axonframework.modelling.command.CreationPolicy
import org.axonframework.spring.stereotype.Aggregate

@Aggregate
class AccountAggregate {
    @AggregateIdentifier
    private var aggregateId: AccountId? = null

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

    @EventSourcingHandler
    fun on(event: NewAccountRegisteredEvent) {
        aggregateId = event.accountId
    }
}

package com.wa.walletaccountant.domain.movementtype

import com.wa.walletaccountant.domain.movementtype.command.RegisterNewMovementTypeCommand
import com.wa.walletaccountant.domain.movementtype.event.NewMovementTypeRegisteredEvent
import com.wa.walletaccountant.domain.movementtype.exception.AccountAndSourceAccountCannotBeTheSameException
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import org.axonframework.commandhandling.CommandHandler
import org.axonframework.eventsourcing.EventSourcingHandler
import org.axonframework.extensions.kotlin.applyEvent
import org.axonframework.modelling.command.AggregateCreationPolicy
import org.axonframework.modelling.command.AggregateIdentifier
import org.axonframework.modelling.command.CreationPolicy
import org.axonframework.spring.stereotype.Aggregate

@Aggregate
class MovementTypeAggregate {
    @AggregateIdentifier
    private var aggregateId: MovementTypeId? = null

    @CommandHandler
    @CreationPolicy(AggregateCreationPolicy.ALWAYS)
    fun on(command: RegisterNewMovementTypeCommand) {
        if (command.sourceAccountId != null && command.accountId == command.sourceAccountId) {
            throw AccountAndSourceAccountCannotBeTheSameException(command.accountId, command.sourceAccountId!!)
        }

        applyEvent(
            NewMovementTypeRegisteredEvent(
                movementTypeId = command.movementTypeId,
                movementAction = command.movementAction,
                accountId = command.accountId,
                sourceAccountId = command.sourceAccountId,
                description = command.description,
                notes = command.notes,
                tagIds = command.tagIds,
            ),
        )
    }

    @EventSourcingHandler
    fun on(event: NewMovementTypeRegisteredEvent) {
        aggregateId = event.movementTypeId
    }




















}
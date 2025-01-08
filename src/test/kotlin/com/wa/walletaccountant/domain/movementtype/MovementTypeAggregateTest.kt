package com.wa.walletaccountant.domain.movementtype

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.command.RegisterNewMovementTypeCommand
import com.wa.walletaccountant.domain.movementtype.event.NewMovementTypeRegisteredEvent
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.test.aggregate.AggregateTestFixture
import org.axonframework.test.aggregate.FixtureConfiguration
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class MovementTypeAggregateTest {
    var fixture: FixtureConfiguration<MovementTypeAggregate>? = null

    @BeforeEach
    fun setUp() {
        fixture = AggregateTestFixture(MovementTypeAggregate::class.java)
    }

    @Test
    fun registerMovementType() {
        val movementTypeId = "c5be2bf8-4ffa-4b3e-a152-518cec206b1d"
        val accountId = "77e52c3d-a0eb-4328-8416-f5e7517120ac"
        val description = "Movement type description"
        val tagId1 = "e661ea45-deba-4e88-98e0-eb0d53ce3ab0"
        val tagId2 = "d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc"

        val command =
            RegisterNewMovementTypeCommand(
                movementTypeId = MovementTypeId.fromString(movementTypeId),
                accountId = AccountId.fromString(accountId),
                sourceAccountId = null,
                movementAction = Debit,
                description = description,
                tagIds = setOf(TagId.fromString(tagId1), TagId.fromString(tagId2)),
                notes = "",
            )

        val event =
            NewMovementTypeRegisteredEvent(
                movementTypeId = MovementTypeId.fromString(movementTypeId),
                accountId = AccountId.fromString(accountId),
                sourceAccountId = null,
                movementAction = Debit,
                description = description,
                tagIds = setOf(TagId.fromString(tagId1), TagId.fromString(tagId2)),
                notes = "",
            )

        fixture!!
            .givenNoPriorActivity()
            .`when`(command)
            .expectSuccessfulHandlerExecution()
            .expectEvents(event)
    }
}
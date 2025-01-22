package com.wa.walletaccountant.domain.ledger

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Currency.EUR
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.EndMonthCommand
import com.wa.walletaccountant.domain.ledger.command.RegisterLedgerMovementCommand
import com.wa.walletaccountant.domain.ledger.command.StartMonthCommand
import com.wa.walletaccountant.domain.ledger.event.LedgerMovementRegisteredEvent
import com.wa.walletaccountant.domain.ledger.event.MonthEndedEvent
import com.wa.walletaccountant.domain.ledger.event.MonthStartedEvent
import com.wa.walletaccountant.domain.ledger.exception.EndBalanceDoesNotMatchCurrentBalanceException
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.MovementId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Credit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.test.aggregate.AggregateTestFixture
import org.axonframework.test.aggregate.FixtureConfiguration
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.math.BigDecimal
import java.time.Month.JANUARY
import java.time.Year
import java.util.stream.Stream

class LedgerAggregrateTest {
    private lateinit var fixture: FixtureConfiguration<LedgerAggregrate>

    companion object {
        private val ledgerId = LedgerId(
            accountId = AccountId.fromString("77e52c3d-a0eb-4328-8416-f5e7517120ac"),
            month = JANUARY,
            year = Year.of(2025),
        )
        private val movementId = MovementId.fromString("9de602c9-e629-4875-958e-384a1a9fbeb4")
        private val movementTypeId = MovementTypeId.fromString("c5be2bf8-4ffa-4b3e-a152-518cec206b1d")
        private val tagId1 = TagId.fromString("e661ea45-deba-4e88-98e0-eb0d53ce3ab0")
        private val tagId2 = TagId.fromString("d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc")

        private val money1 = Money(BigDecimal.valueOf(10), EUR)
        private val money2 = Money(BigDecimal.valueOf(15), EUR)
        private val money3 = Money(BigDecimal.valueOf(25), EUR)
        private val money4 = Money(BigDecimal.valueOf(-5), EUR)

        val monthStartedEvent =
            MonthStartedEvent(
                ledgerId = ledgerId,
                startBalance = money1,
            )

        val ledgerMovementRegisteredCreditEvent =
            LedgerMovementRegisteredEvent(
                ledgerId = ledgerId,
                movementId = movementId,
                movementTypeId = null,
                action = Credit,
                amount = money2,
                date = Date.now(),
                sourceAccountId = null,
                description = "a credit",
                notes = "no notes",
                tagIds = setOf(tagId1),
            )

        val ledgerMovementRegisteredDebitEvent =
            LedgerMovementRegisteredEvent(
                ledgerId = ledgerId,
                movementId = movementId,
                movementTypeId = movementTypeId,
                action = Debit,
                amount = money2,
                date = Date.now(),
                sourceAccountId = null,
                description = "a debit",
                notes = "no notes",
                tagIds = setOf(tagId1, tagId2),
            )

        @JvmStatic
        fun movementBalanceData(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    "Debit",
                    ledgerMovementRegisteredDebitEvent,
                    money4
                ),
                Arguments.of(
                    "Credit",
                    ledgerMovementRegisteredCreditEvent,
                    money3
                )
            )
    }

    @BeforeEach
    fun setUp() {
        fixture = AggregateTestFixture(LedgerAggregrate::class.java)
    }

    @Test
    fun startMonthSucceeds() {
        fixture
            .givenNoPriorActivity()
            .`when`(
                StartMonthCommand(
                    ledgerId = ledgerId,
                    startBalance = money1,
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(monthStartedEvent)
    }

    @Test
    fun endMonthSucceeds() {
        fixture
            .given(monthStartedEvent)
            .`when`(
                EndMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = money1,
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(
                MonthEndedEvent(
                    ledgerId = ledgerId,
                    endBalance = money1,
                )
            )
    }

    @Test
    fun endMonthFailsWithMismatchedBalance() {
        fixture
            .given(monthStartedEvent)
            .`when`(
                EndMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = money3,
                )
            )
            .expectException(EndBalanceDoesNotMatchCurrentBalanceException::class.java)
            .expectExceptionMessage(
                "Ledger: Ledger balance does not match expected end of month balance. [ledgerId: " +
                        "7e80cc3b-464c-3015-9c15-312888c8371c] [accountId: 77e52c3d-a0eb-4328-8416-f5e7517120ac] " +
                        "[month: 1] [year: 2025] [currentBalance: 10 EUR] [endBalance: 25 EUR]"
            )
            .expectNoEvents()
    }

    @Test
    fun registerLedgerMovementSucceeds() {
        fixture
            .given(monthStartedEvent)
            .`when`(
                RegisterLedgerMovementCommand(
                    ledgerId = ledgerId,
                    movementId = movementId,
                    movementTypeId = movementTypeId,
                    action = Debit,
                    amount = money2,
                    date = Date.now(),
                    sourceAccountId = null,
                    description = "a debit",
                    notes = "no notes",
                    tagIds = setOf(tagId1, tagId2),
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(ledgerMovementRegisteredDebitEvent)
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("movementBalanceData")
    fun registerLedgerMovementAppliesCorrectBalanceCalculation(
        testName: String,
        movementEvent: LedgerMovementRegisteredEvent,
        expectedBalance: Money,
    ) {
        fixture
            .given(monthStartedEvent)
            .andGiven(movementEvent)
            .`when`(
                EndMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = expectedBalance,
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(
                MonthEndedEvent(
                    ledgerId = ledgerId,
                    endBalance = expectedBalance,
                )
            )
    }
}
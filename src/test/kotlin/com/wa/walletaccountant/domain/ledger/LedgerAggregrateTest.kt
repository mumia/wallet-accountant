package com.wa.walletaccountant.domain.ledger

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.CloseBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.command.OpenBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.command.RegisterTransactionCommand
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceClosedEvent
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceOpenedEvent
import com.wa.walletaccountant.domain.ledger.event.TransactionRegisteredEvent
import com.wa.walletaccountant.domain.ledger.exception.CloseBalanceDoesNotMatchCurrentBalanceException
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
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
        private val transactionId = TransactionId.fromString("9de602c9-e629-4875-958e-384a1a9fbeb4")
        private val movementTypeId = MovementTypeId.fromString("c5be2bf8-4ffa-4b3e-a152-518cec206b1d")
        private val tagId1 = TagId.fromString("e661ea45-deba-4e88-98e0-eb0d53ce3ab0")
        private val tagId2 = TagId.fromString("d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc")

        val monthBalanceOpenedEvent =
            MonthBalanceOpenedEvent(
                ledgerId = ledgerId,
                startBalance = Money(amount = 10),
            )

        val ledgerMovementRegisteredCreditEvent =
            TransactionRegisteredEvent(
                ledgerId = ledgerId,
                transactionId = transactionId,
                movementTypeId = null,
                action = Credit,
                amount = Money(amount = 15),
                date = Date.now(),
                sourceAccountId = null,
                description = "a credit",
                notes = "no notes",
                tagIds = HashSet(setOf(tagId1)),
            )

        val ledgerMovementRegisteredDebitEvent =
            TransactionRegisteredEvent(
                ledgerId = ledgerId,
                transactionId = transactionId,
                movementTypeId = movementTypeId,
                action = Debit,
                amount = Money(amount = -15),
                date = Date.now(),
                sourceAccountId = null,
                description = "a debit",
                notes = "no notes",
                tagIds = HashSet(setOf(tagId1, tagId2)),
            )

        @JvmStatic
        fun movementBalanceData(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    "Debit",
                    ledgerMovementRegisteredDebitEvent,
                    Money(amount = -5)
                ),
                Arguments.of(
                    "Credit",
                    ledgerMovementRegisteredCreditEvent,
                    Money(amount = 25)
                ),
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
                OpenBalanceForMonthCommand(
                    ledgerId = ledgerId,
                    startBalance = Money(amount = 10),
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(monthBalanceOpenedEvent)
    }

    @Test
    fun endMonthSucceeds() {
        fixture
            .given(monthBalanceOpenedEvent)
            .`when`(
                CloseBalanceForMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = Money(amount = 10),
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(
                MonthBalanceClosedEvent(
                    ledgerId = ledgerId,
                    closeBalance = Money(amount = 10),
                )
            )
    }

    @Test
    fun endMonthFailsWithMismatchedBalance() {
        fixture
            .given(monthBalanceOpenedEvent)
            .`when`(
                CloseBalanceForMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = Money(amount = 25),
                )
            )
            .expectException(CloseBalanceDoesNotMatchCurrentBalanceException::class.java)
            .expectExceptionMessage(
                "Ledger: Ledger balance does not match expected end of month balance. [ledgerId: " +
                        "7e80cc3b-464c-3015-9c15-312888c8371c] [accountId: 77e52c3d-a0eb-4328-8416-f5e7517120ac] " +
                        "[month: 1] [year: 2025] [currentBalance: 10.00] [endBalance: 25.00]"
            )
            .expectNoEvents()
    }

    @Test
    fun registerLedgerMovementSucceeds() {
        fixture
            .given(monthBalanceOpenedEvent)
            .`when`(
                RegisterTransactionCommand(
                    ledgerId = ledgerId,
                    transactionId = transactionId,
                    movementTypeId = movementTypeId,
                    amount = Money(amount = -15),
                    date = Date.now(),
                    sourceAccountId = null,
                    description = "a debit",
                    notes = "no notes",
                    tagIds = HashSet(setOf(tagId1, tagId2)),
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(ledgerMovementRegisteredDebitEvent)
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("movementBalanceData")
    fun registerLedgerMovementAppliesCorrectBalanceCalculation(
        testName: String,
        movementEvent: TransactionRegisteredEvent,
        expectedBalance: Money,
    ) {
        fixture
            .given(monthBalanceOpenedEvent)
            .andGiven(movementEvent)
            .`when`(
                CloseBalanceForMonthCommand(
                    ledgerId = ledgerId,
                    endBalance = expectedBalance,
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectEvents(
                MonthBalanceClosedEvent(
                    ledgerId = ledgerId,
                    closeBalance = expectedBalance,
                )
            )
    }
}
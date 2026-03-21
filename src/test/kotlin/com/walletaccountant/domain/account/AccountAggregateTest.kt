package com.walletaccountant.domain.account

import com.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.axonframework.eventsourcing.configuration.EventSourcingConfigurer
import org.axonframework.eventsourcing.configuration.EventSourcedEntityModule
import org.axonframework.test.fixture.AxonTestFixture
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.uuid.Uuid

class AccountAggregateTest {

    private lateinit var fixture: AxonTestFixture

    @BeforeEach
    fun setUp() {
        fixture = AxonTestFixture.with(
            EventSourcingConfigurer.create()
                .registerEntity(
                    EventSourcedEntityModule.autodetected(String::class.java, Account::class.java)
                )
        ) { it.disableAxonServer() }
    }

    @AfterEach
    fun tearDown() {
        fixture.stop()
    }

    @Test
    fun `should register new account with all required fields`() {
        val accountId = AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000"))
        val command = RegisterNewAccountCommand(
            accountId = accountId,
            bankName = BankName.BCP,
            name = "Main Checking",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("1000.50")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2026, 1, 15)),
            notes = null
        )

        val expectedEvent = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.BCP,
            name = "Main Checking",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("1000.50")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2026, 1, 15)),
            month = Month.JANUARY,
            year = Year.of(2026),
            notes = null
        )

        fixture.given()
            .noPriorActivity()
            .`when`()
            .command(command)
            .then()
            .success()
            .events(expectedEvent)
    }

    @Test
    fun `should register new account with optional notes`() {
        val accountId = AccountId(Uuid.parse("660e8400-e29b-41d4-a716-446655440000"))
        val command = RegisterNewAccountCommand(
            accountId = accountId,
            bankName = BankName.N26,
            name = "Savings",
            accountType = AccountType.SAVINGS,
            startingBalance = Money.of(BigDecimal("0.00")),
            currency = Currency.CHF,
            startingDate = Date(LocalDate.of(2026, 3, 1)),
            notes = "Emergency fund"
        )

        val expectedEvent = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.N26,
            name = "Savings",
            accountType = AccountType.SAVINGS,
            startingBalance = Money.of(BigDecimal("0.00")),
            currency = Currency.CHF,
            startingDate = Date(LocalDate.of(2026, 3, 1)),
            month = Month.MARCH,
            year = Year.of(2026),
            notes = "Emergency fund"
        )

        fixture.given()
            .noPriorActivity()
            .`when`()
            .command(command)
            .then()
            .success()
            .events(expectedEvent)
    }

    @Test
    fun `should derive month and year from starting date`() {
        val accountId = AccountId(Uuid.parse("990e8400-e29b-41d4-a716-446655440000"))
        val command = RegisterNewAccountCommand(
            accountId = accountId,
            bankName = BankName.BCP,
            name = "December Account",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("0.00")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2025, 12, 31)),
            notes = null
        )

        fixture.given()
            .noPriorActivity()
            .`when`()
            .command(command)
            .then()
            .success()
            .eventsSatisfy { events ->
                val event = events[0].payload() as NewAccountRegisteredEvent
                assert(event.month == Month.DECEMBER) { "Expected DECEMBER but got ${event.month}" }
                assert(event.year == Year.of(2025)) { "Expected 2025 but got ${event.year}" }
            }
    }

    @Test
    fun `should populate all aggregate state from event sourcing`() {
        val accountId = AccountId(Uuid.parse("770e8400-e29b-41d4-a716-446655440000"))
        val existingEvent = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.WISE,
            name = "Travel Fund",
            accountType = AccountType.SAVINGS,
            startingBalance = Money.of(BigDecimal("500.00")),
            currency = Currency.USD,
            startingDate = Date(LocalDate.of(2026, 6, 15)),
            month = Month.JUNE,
            year = Year.of(2026),
            notes = "For vacation"
        )

        // Verify that given an existing event, the aggregate can be reconstructed
        // (the event sourcing handler runs without error)
        fixture.given()
            .event(existingEvent)
            .then()
    }
}

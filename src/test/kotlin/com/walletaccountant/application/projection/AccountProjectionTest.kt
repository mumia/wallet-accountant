package com.walletaccountant.application.projection

import com.walletaccountant.application.port.out.AccountReadModelRepository
import com.walletaccountant.application.readmodel.account.AccountReadModel
import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import io.mockk.every
import io.mockk.mockk
import io.mockk.slot
import io.mockk.verify
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.uuid.Uuid

class AccountProjectionTest {

    private lateinit var repository: AccountReadModelRepository
    private lateinit var projection: AccountProjection

    @BeforeEach
    fun setUp() {
        repository = mockk()
        projection = AccountProjection(repository)
    }

    @Test
    fun `should project NewAccountRegisteredEvent to read model`() {
        val accountId = AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000"))
        val event = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.BCP,
            name = "Main Checking",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("1000.50")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2026, 1, 15)),
            month = Month.JANUARY,
            year = Year.of(2026),
            notes = "Primary account"
        )

        val savedSlot = slot<AccountReadModel>()
        every { repository.save(capture(savedSlot)) } answers { savedSlot.captured }

        projection.on(event)

        val saved = savedSlot.captured
        assertEquals("550e8400-e29b-41d4-a716-446655440000", saved.accountId)
        assertEquals(BankName.BCP, saved.bankName)
        assertEquals("Main Checking", saved.name)
        assertEquals(AccountType.CHECKING, saved.accountType)
        assertEquals(Money.of(BigDecimal("1000.50")), saved.startingBalance)
        assertEquals(Currency.EUR, saved.currency)
        assertEquals(LocalDate.of(2026, 1, 15), saved.startingDate)
        assertEquals(Month.JANUARY, saved.month)
        assertEquals(Year.of(2026), saved.year)
        assertEquals("Primary account", saved.notes)
    }

    @Test
    fun `should project event with null notes`() {
        val accountId = AccountId(Uuid.parse("660e8400-e29b-41d4-a716-446655440000"))
        val event = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.N26,
            name = "Savings",
            accountType = AccountType.SAVINGS,
            startingBalance = Money.of(BigDecimal("0.00")),
            currency = Currency.CHF,
            startingDate = Date(LocalDate.of(2026, 3, 1)),
            month = Month.MARCH,
            year = Year.of(2026),
            notes = null
        )

        val savedSlot = slot<AccountReadModel>()
        every { repository.save(capture(savedSlot)) } answers { savedSlot.captured }

        projection.on(event)

        assertNull(savedSlot.captured.notes)
    }

    @Test
    fun `should call repository save exactly once`() {
        val accountId = AccountId(Uuid.parse("770e8400-e29b-41d4-a716-446655440000"))
        val event = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.WISE,
            name = "Travel",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("500.00")),
            currency = Currency.USD,
            startingDate = Date(LocalDate.of(2026, 6, 15)),
            month = Month.JUNE,
            year = Year.of(2026),
            notes = null
        )

        every { repository.save(any()) } answers { firstArg() }

        projection.on(event)

        verify(exactly = 1) { repository.save(any()) }
    }

    @Test
    fun `should produce same read model on replay (idempotent mapping)`() {
        val accountId = AccountId(Uuid.parse("880e8400-e29b-41d4-a716-446655440000"))
        val event = NewAccountRegisteredEvent(
            accountId = accountId,
            bankName = BankName.BCP,
            name = "Replay Test",
            accountType = AccountType.SAVINGS,
            startingBalance = Money.of(BigDecimal("250.00")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2026, 2, 28)),
            month = Month.FEBRUARY,
            year = Year.of(2026),
            notes = "Test notes"
        )

        val savedModels = mutableListOf<AccountReadModel>()
        every { repository.save(capture(savedModels)) } answers { savedModels.last() }

        projection.on(event)
        projection.on(event)

        assertEquals(2, savedModels.size)
        assertEquals(savedModels[0], savedModels[1])
    }
}

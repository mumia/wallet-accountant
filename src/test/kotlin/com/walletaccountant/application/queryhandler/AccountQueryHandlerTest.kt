package com.walletaccountant.application.queryhandler

import com.walletaccountant.application.port.out.AccountReadModelRepository
import com.walletaccountant.application.readmodel.account.AccountReadModel
import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.account.query.GetAccountByIdQuery
import com.walletaccountant.domain.account.query.GetAllAccountsQuery
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Money
import io.mockk.every
import io.mockk.mockk
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue
import kotlin.uuid.Uuid

class AccountQueryHandlerTest {

    private lateinit var repository: AccountReadModelRepository
    private lateinit var queryHandler: AccountQueryHandler

    @BeforeEach
    fun setUp() {
        repository = mockk()
        queryHandler = AccountQueryHandler(repository)
    }

    private fun createReadModel(
        accountId: String = "550e8400-e29b-41d4-a716-446655440000",
        bankName: BankName = BankName.BCP,
        name: String = "Main Checking",
        accountType: AccountType = AccountType.CHECKING,
        startingBalance: Money = Money.of(BigDecimal("1000.50")),
        currency: Currency = Currency.EUR,
        startingDate: LocalDate = LocalDate.of(2026, 1, 15),
        month: Month = Month.JANUARY,
        year: Year = Year.of(2026),
        notes: String? = null
    ) = AccountReadModel(
        accountId = accountId,
        bankName = bankName,
        name = name,
        accountType = accountType,
        startingBalance = startingBalance,
        currency = currency,
        startingDate = startingDate,
        month = month,
        year = year,
        notes = notes
    )

    @Test
    fun `should return account read model when found by ID`() {
        val readModel = createReadModel()
        every { repository.findByAccountId("550e8400-e29b-41d4-a716-446655440000") } returns readModel

        val query = GetAccountByIdQuery(AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000")))
        val result = queryHandler.handle(query)

        assertEquals(readModel, result)
    }

    @Test
    fun `should return null when account not found by ID`() {
        every { repository.findByAccountId(any()) } returns null

        val query = GetAccountByIdQuery(AccountId(Uuid.parse("990e8400-e29b-41d4-a716-446655440000")))
        val result = queryHandler.handle(query)

        assertNull(result)
    }

    @Test
    fun `should return all accounts`() {
        val readModel1 = createReadModel(accountId = "550e8400-e29b-41d4-a716-446655440000", name = "Account 1")
        val readModel2 = createReadModel(accountId = "660e8400-e29b-41d4-a716-446655440000", name = "Account 2")
        every { repository.findAll() } returns listOf(readModel1, readModel2)

        val result = queryHandler.handle(GetAllAccountsQuery())

        assertEquals(2, result.size)
        assertEquals(readModel1, result[0])
        assertEquals(readModel2, result[1])
    }

    @Test
    fun `should return empty list when no accounts exist`() {
        every { repository.findAll() } returns emptyList()

        val result = queryHandler.handle(GetAllAccountsQuery())

        assertTrue(result.isEmpty())
    }
}

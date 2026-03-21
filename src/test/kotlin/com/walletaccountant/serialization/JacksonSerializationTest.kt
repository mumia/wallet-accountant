package com.walletaccountant.serialization

import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.SerializationFeature
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.test.assertEquals
import kotlin.uuid.Uuid

class JacksonSerializationTest {

    private lateinit var objectMapper: ObjectMapper

    @BeforeEach
    fun setUp() {
        objectMapper = jacksonObjectMapper()
            .registerModule(JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
    }

    private inline fun <reified T> deserialize(json: String): T =
        objectMapper.readValue(json, T::class.java)

    @Test
    fun `should serialize and deserialize AccountId`() {
        val id = AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000"))

        val json = objectMapper.writeValueAsString(id)
        val deserialized = deserialize<AccountId>(json)

        assertEquals(id, deserialized)
    }

    @Test
    fun `should serialize AccountId as string`() {
        val id = AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000"))

        val json = objectMapper.writeValueAsString(id)

        assertEquals("\"550e8400-e29b-41d4-a716-446655440000\"", json)
    }

    @Test
    fun `should serialize and deserialize Money`() {
        val money = Money.of(BigDecimal("100.50"))

        val json = objectMapper.writeValueAsString(money)
        val deserialized = deserialize<Money>(json)

        assertEquals(money, deserialized)
    }

    @Test
    fun `should serialize and deserialize Currency`() {
        val currency = Currency.EUR

        val json = objectMapper.writeValueAsString(currency)
        val deserialized = deserialize<Currency>(json)

        assertEquals(currency, deserialized)
    }

    @Test
    fun `should serialize and deserialize BankName`() {
        val bankName = BankName.BCP

        val json = objectMapper.writeValueAsString(bankName)
        val deserialized = deserialize<BankName>(json)

        assertEquals(bankName, deserialized)
    }

    @Test
    fun `should serialize and deserialize AccountType`() {
        val accountType = AccountType.CHECKING

        val json = objectMapper.writeValueAsString(accountType)
        val deserialized = deserialize<AccountType>(json)

        assertEquals(accountType, deserialized)
    }

    @Test
    fun `should serialize and deserialize Date`() {
        val date = Date(LocalDate.of(2026, 1, 15))

        val json = objectMapper.writeValueAsString(date)
        val deserialized = deserialize<Date>(json)

        assertEquals(date, deserialized)
    }

    @Test
    fun `should serialize and deserialize Month`() {
        val month = Month.JANUARY

        val json = objectMapper.writeValueAsString(month)
        val deserialized = deserialize<Month>(json)

        assertEquals(month, deserialized)
    }

    @Test
    fun `should serialize and deserialize Year`() {
        val year = Year.of(2026)

        val json = objectMapper.writeValueAsString(year)
        val deserialized = deserialize<Year>(json)

        assertEquals(year, deserialized)
    }

    @Test
    fun `should serialize Date value as ISO string`() {
        val date = Date(LocalDate.of(2026, 1, 15))

        val json = objectMapper.writeValueAsString(date)

        assertEquals("{\"value\":\"2026-01-15\"}", json)
    }

    @Test
    fun `should serialize Month as string name`() {
        val month = Month.MARCH

        val json = objectMapper.writeValueAsString(month)

        assertEquals("\"MARCH\"", json)
    }
}

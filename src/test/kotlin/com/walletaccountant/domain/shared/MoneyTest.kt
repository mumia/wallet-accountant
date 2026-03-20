package com.walletaccountant.domain.shared

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import java.math.BigDecimal
import kotlin.test.assertEquals

class MoneyTest {

    @Test
    fun `should create money with exactly 2 decimal places`() {
        val money = Money.of(BigDecimal("100.50"))

        assertEquals(BigDecimal("100.50"), money.amount)
        assertEquals(2, money.amount.scale())
    }

    @Test
    fun `should enforce scale 2 when created with integer`() {
        val money = Money.of(BigDecimal("100"))

        assertEquals(BigDecimal("100.00"), money.amount)
        assertEquals(2, money.amount.scale())
    }

    @Test
    fun `should enforce scale 2 when created with single decimal`() {
        val money = Money.of(BigDecimal("100.5"))

        assertEquals(BigDecimal("100.50"), money.amount)
        assertEquals(2, money.amount.scale())
    }

    @Test
    fun `should reject amount with more than 2 decimal places`() {
        assertThrows<IllegalArgumentException> {
            Money.of(BigDecimal("100.123"))
        }
    }

    @Test
    fun `should support zero amount`() {
        val money = Money.of(BigDecimal("0.00"))

        assertEquals(BigDecimal("0.00"), money.amount)
    }

    @Test
    fun `should support negative amount`() {
        val money = Money.of(BigDecimal("-50.25"))

        assertEquals(BigDecimal("-50.25"), money.amount)
    }

    @Test
    fun `should have value equality`() {
        val money1 = Money.of(BigDecimal("100.50"))
        val money2 = Money.of(BigDecimal("100.50"))

        assertEquals(money1, money2)
    }
}

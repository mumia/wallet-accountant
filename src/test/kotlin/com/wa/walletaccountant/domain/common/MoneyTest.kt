package com.wa.walletaccountant.domain.common

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.math.BigDecimal
import java.util.stream.Stream

class MoneyTest {
    @Test
    fun shouldBeSame() {
        val money1 = Money(BigDecimal.valueOf(10.23), Currency.USD)
        val money2 = Money(BigDecimal.valueOf(10.23), Currency.USD)

        assertEquals(money1, money2)
        assertEquals(money1.hashCode(), money2.hashCode())
        assertTrue(money1 == money2)
        assertEquals("10.23 USD", money1.toString())
    }

    @ParameterizedTest
    @MethodSource("differentMoney")
    fun shouldNotBeSame(
        money1: Money,
        money2: Money,
    ) {
        assertNotEquals(money1, money2)
        assertNotEquals(money1.hashCode(), money2.hashCode())
        assertFalse { money1 == money2 }
    }

    companion object {
        @JvmStatic
        fun differentMoney(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    Money(BigDecimal.valueOf(10.0), Currency.USD),
                    Money(BigDecimal.valueOf(10.0), Currency.EUR),
                ),
                Arguments.of(
                    Money(BigDecimal.valueOf(20.0), Currency.USD),
                    Money(BigDecimal.valueOf(10.0), Currency.USD),
                ),
            )
    }
}

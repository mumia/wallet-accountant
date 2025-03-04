package com.wa.walletaccountant.domain.common

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.util.stream.Stream

class DateTest {
    @ParameterizedTest
    @MethodSource("sameDate")
    fun shouldBeSame(date1: Date, date2: Date) {
        val date1 = Date.fromString("2025-01-01")
        val date2 = Date.fromString("2025-01-01")

        assertEquals(date1, date2)
        assertEquals(date1.hashCode(), date2.hashCode())
        assertTrue(date1 == date2)
        assertEquals("2025-01-01", date1.toString())
    }

    @ParameterizedTest
    @MethodSource("differentDate")
    fun shouldNotBeSame(date1: Date, date2: Date) {
        assertNotEquals(date1, date2)
        assertNotEquals(date1.hashCode(), date2.hashCode())
        assertFalse { date1 == date2 }
    }

    companion object {
        @JvmStatic
        fun sameDate(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    Date.fromString("2025-01-01"),
                    Date.fromString("2025-01-01"),
                ),
                Arguments.of(
                    Date.now(),
                    Date.now(),
                ),
            )

        @JvmStatic
        fun differentDate(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    Date.fromString("2025-01-01"),
                    Date.fromString("2024-01-01"),
                ),
                Arguments.of(
                    Date.fromString("2025-01-01"),
                    Date.fromString("2025-02-01"),
                ),
                Arguments.of(
                    Date.fromString("2025-01-02"),
                    Date.fromString("2025-01-01"),
                ),
            )
    }
}
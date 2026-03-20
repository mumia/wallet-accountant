package com.walletaccountant.domain.shared

import org.junit.jupiter.api.Test
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.test.assertEquals

class DateTest {

    @Test
    fun `should create date from LocalDate`() {
        val date = Date(LocalDate.of(2026, 1, 15))

        assertEquals(LocalDate.of(2026, 1, 15), date.value)
    }

    @Test
    fun `should provide month accessor`() {
        val date = Date(LocalDate.of(2026, 3, 20))

        assertEquals(Month.MARCH, date.month)
    }

    @Test
    fun `should provide year accessor`() {
        val date = Date(LocalDate.of(2026, 3, 20))

        assertEquals(Year.of(2026), date.year)
    }

    @Test
    fun `should have value equality`() {
        val date1 = Date(LocalDate.of(2026, 1, 15))
        val date2 = Date(LocalDate.of(2026, 1, 15))

        assertEquals(date1, date2)
    }
}

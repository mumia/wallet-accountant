package com.wa.walletaccountant.domain.common

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import java.time.Month
import java.time.Year
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeFormatter.ISO_LOCAL_DATE
import java.time.temporal.ChronoUnit

data class Date(
    private val value: ZonedDateTime,
) {
    companion object {
        val UTC: ZoneId = ZoneId.of("UTC")

        fun now(): Date = Date(ZonedDateTime.now().truncatedTo(ChronoUnit.DAYS))

        @JsonCreator
        fun fromString(stringValue: String): Date =
            Date(
                ZonedDateTime.parse(
                    stringValue + "T00:00:00Z",
                    DateTimeFormatter.ISO_INSTANT.withZone(UTC),
                ),
            )

        fun nextMonth(month: Month, year: Year): Date =
            Date(
                ZonedDateTime
                    .of(year.value, month.value, 1, 0, 0, 0, 0, UTC)
                    .plusMonths(1)
            )
    }

    fun year(): Year = Year.of(value.year)

    fun month(): Month = Month.of(value.monthValue)

    fun day(): Int = value.dayOfMonth

    @JsonValue
    override fun toString(): String = value.format(ISO_LOCAL_DATE)
}

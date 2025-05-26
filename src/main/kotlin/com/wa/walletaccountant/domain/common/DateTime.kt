package com.wa.walletaccountant.domain.common

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import java.time.Month
import java.time.Year
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter.ISO_INSTANT
import java.time.temporal.ChronoUnit

data class DateTime
private constructor(
    private val value: ZonedDateTime,
) {
    companion object {
        val UTC: ZoneId = ZoneId.of("UTC")

        fun now(): DateTime = DateTime(ZonedDateTime.now(Date.UTC).truncatedTo(ChronoUnit.SECONDS))

        @JsonCreator
        fun fromString(stringValue: String): DateTime =
            DateTime(ZonedDateTime.parse(stringValue, ISO_INSTANT.withZone(UTC)).truncatedTo(ChronoUnit.SECONDS))
    }

    fun year(): Year = Year.of(value.year)

    fun month(): Month = Month.of(value.monthValue)

    @JsonValue
    override fun toString(): String = value.format(ISO_INSTANT)

    fun timestamp(): Long = value.toEpochSecond()
}

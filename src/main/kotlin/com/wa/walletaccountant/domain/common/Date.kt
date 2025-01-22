package com.wa.walletaccountant.domain.common

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.time.format.DateTimeFormatter.ISO_LOCAL_DATE
import java.time.temporal.ChronoUnit

data class Date(
    val value: ZonedDateTime,
) {
    companion object {
        fun now(): Date = Date(ZonedDateTime.now().truncatedTo(ChronoUnit.DAYS))

        @JsonCreator
        fun fromString(stringValue: String): Date =
            Date(
                ZonedDateTime.parse(
                    stringValue + "T00:00:00Z",
                    DateTimeFormatter.ISO_INSTANT.withZone(ZoneId.of("UTC")),
                ),
            )
    }

    @JsonValue
    override fun toString(): String = value.format(ISO_LOCAL_DATE)
}

package com.walletaccountant.domain.shared

import com.fasterxml.jackson.annotation.JsonIgnore
import java.time.LocalDate
import java.time.Month
import java.time.Year

data class Date(val value: LocalDate) {
    @get:JsonIgnore
    val month: Month get() = value.month

    @get:JsonIgnore
    val year: Year get() = Year.of(value.year)
}

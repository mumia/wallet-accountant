package com.wa.walletaccountant.adapter.`in`.web.ledger.request

import com.wa.walletaccountant.adapter.`in`.web.constraint.EnumConstraint
import com.wa.walletaccountant.adapter.`in`.web.constraint.ValidAccountYearConstraint
import jakarta.validation.constraints.NotEmpty
import jakarta.validation.constraints.NotNull
import org.hibernate.validator.constraints.UUID
import org.springframework.format.annotation.NumberFormat
import org.springframework.format.annotation.NumberFormat.Style.CURRENCY
import java.math.BigDecimal
import java.time.Month

data class CloseBalanceForMonthRequest(
    @field:NotEmpty
    @field:UUID
    val accountId: String,

    @field:NotEmpty
    @field:EnumConstraint(
        enumClass = Month::class,
        message = "Expected a valid month name"
    )
    val month: String,

    @field:ValidAccountYearConstraint
    val year: Int,

    @field:NotNull
    @field:NumberFormat(style = CURRENCY)
    val endBalance: BigDecimal,
)

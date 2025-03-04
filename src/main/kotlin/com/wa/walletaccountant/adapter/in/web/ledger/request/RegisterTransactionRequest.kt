package com.wa.walletaccountant.adapter.`in`.web.ledger.request

import jakarta.validation.constraints.NotEmpty
import jakarta.validation.constraints.NotNull
import jakarta.validation.constraints.Pattern
import org.hibernate.validator.constraints.UUID
import org.springframework.format.annotation.NumberFormat
import org.springframework.format.annotation.NumberFormat.Style.CURRENCY
import java.math.BigDecimal

data class RegisterTransactionRequest(
    @field:NotEmpty
    @field:UUID
    val accountId: String,
    @field:UUID(allowEmpty = true)
    val movementTypeId: String?,
    @field:NotNull
    @field:NumberFormat(style = CURRENCY)
    val amount: BigDecimal,
    @field:NotEmpty
    @field:Pattern(regexp = "([0-9]{4}-[0-9]{2}-[0-9]{2})", message = "Expected date format is YYYY-MM-DD")
    val date: String,
    @field:UUID(allowEmpty = true)
    val sourceAccountId: String?,
    @field:NotEmpty
    val description: String,
    val notes: String?,
    @field:NotEmpty
    val tagIds: Set<@UUID String>,
)

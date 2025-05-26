package com.wa.walletaccountant.adapter.`in`.web.account.request

import com.wa.walletaccountant.adapter.`in`.web.constraint.DateDayFormatConstraint
import com.wa.walletaccountant.adapter.`in`.web.constraint.EnumConstraint
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.common.Currency
import jakarta.validation.constraints.Min
import jakarta.validation.constraints.NotEmpty
import java.math.BigDecimal

data class NewAccountRequest(
    @field:NotEmpty
    @field:EnumConstraint(
        enumClass = BankName::class,
        message = "Invalid bank name found, expected one of {values}",
    )
    val bankName: String,
    @field:NotEmpty
    val name: String,
    @field:NotEmpty
    @field:EnumConstraint(
        enumClass = AccountType::class,
        message = "Invalid account type found, expected one of {values}",
    )
    val accountType: String,
    @field:Min(value = 0)
    val startingBalance: BigDecimal,
    @field:NotEmpty
    @field:DateDayFormatConstraint
    val startingBalanceDate: String,
    @field:NotEmpty
    @field:EnumConstraint(
        enumClass = Currency::class,
        message = "Invalid currency found, expected one of {values}",
    )
    val currency: String,
    val notes: String?,
)

package com.wa.walletaccountant.adapter.`in`.web.account.request

import com.wa.walletaccountant.adapter.`in`.web.constraint.EnumConstraint
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.common.Currency
import jakarta.validation.constraints.Min
import jakarta.validation.constraints.NotEmpty
import jakarta.validation.constraints.Pattern

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
    val startingBalance: Double,
    @field:NotEmpty
    @field:Pattern(regexp = "([0-9]{4}-[0-9]{2}-[0-9]{2})", message = "Expected date format is YYYY-MM-DD")
    val startingBalanceDate: String,
    @field:NotEmpty
    @field:EnumConstraint(
        enumClass = Currency::class,
        message = "Invalid currency found, expected one of {values}",
    )
    val currency: String,
    val notes: String?,
)

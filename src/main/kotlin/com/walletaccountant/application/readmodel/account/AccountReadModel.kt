package com.walletaccountant.application.readmodel.account

import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Money
import org.springframework.data.annotation.Id
import org.springframework.data.mongodb.core.mapping.Document
import java.time.LocalDate
import java.time.Month
import java.time.Year

@Document(collection = "accounts")
data class AccountReadModel(
    @Id val accountId: String,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val currency: Currency,
    val startingDate: LocalDate,
    val month: Month,
    val year: Year,
    val notes: String? = null
)

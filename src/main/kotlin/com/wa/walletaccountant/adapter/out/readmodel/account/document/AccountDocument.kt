package com.wa.walletaccountant.adapter.out.readmodel.account.document

import com.wa.walletaccountant.application.model.account.AccountModel.ActiveMonth
import com.wa.walletaccountant.domain.account.account.AccountType
import com.wa.walletaccountant.domain.account.account.BankName
import com.wa.walletaccountant.domain.common.Currency
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import org.springframework.data.annotation.TypeAlias
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.data.mongodb.core.mapping.MongoId

@Document("account")
@TypeAlias("Account")
data class AccountDocument(
    @MongoId
    val aggregateId: String,
    val bankName: BankName,
    val name: String,
    val accountType: AccountType,
    val startingBalance: Money,
    val startingBalanceDate: Date,
    val currency: Currency,
    val notes: String?,
    val activeMonth: ActiveMonth,
)

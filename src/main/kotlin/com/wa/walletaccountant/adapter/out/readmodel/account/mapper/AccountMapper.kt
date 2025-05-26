package com.wa.walletaccountant.adapter.out.readmodel.account.mapper

import com.wa.walletaccountant.adapter.out.readmodel.account.document.AccountDocument
import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.domain.account.account.AccountId
import org.springframework.stereotype.Service

@Service
object AccountMapper {
    fun toDocument(model: AccountModel): AccountDocument =
        AccountDocument(
            aggregateId = model.accountId.id(),
            bankName = model.bankName,
            name = model.name,
            accountType = model.accountType,
            startingBalance = model.startingBalance,
            startingBalanceDate = model.startingBalanceDate,
            currency = model.currency,
            notes = model.notes,
            activeMonth = model.activeMonth,
        )

    fun toModel(document: AccountDocument): AccountModel =
        AccountModel(
            aggregateId = document.aggregateId,
            accountId = AccountId.fromString(document.aggregateId),
            bankName = document.bankName,
            name = document.name,
            accountType = document.accountType,
            startingBalance = document.startingBalance,
            startingBalanceDate = document.startingBalanceDate,
            currency = document.currency,
            notes = document.notes,
            activeMonth = document.activeMonth,
        )
}

package com.wa.walletaccountant.adapter.out.readmodel.account.mapper

import com.wa.walletaccountant.adapter.out.readmodel.account.document.AccountDocument
import com.wa.walletaccountant.application.model.account.AccountModel
import org.springframework.stereotype.Service

@Service
object AccountMapper {
    fun toDocument(model: AccountModel): AccountDocument =
        AccountDocument(
            accountId = model.accountId,
            bankName = model.bankName,
            name = model.name,
            accountType = model.accountType,
            startingBalance = model.startingBalance,
            startingBalanceDate = model.startingBalanceDate,
            currency = model.currency,
            notes = model.notes,
        )

    fun toModel(document: AccountDocument): AccountModel =
        AccountModel(
            accountId = document.accountId,
            bankName = document.bankName,
            name = document.name,
            accountType = document.accountType,
            startingBalance = document.startingBalance,
            startingBalanceDate = document.startingBalanceDate,
            currency = document.currency,
            notes = document.notes,
        )
}

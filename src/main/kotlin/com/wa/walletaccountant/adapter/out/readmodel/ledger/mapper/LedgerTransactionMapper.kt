package com.wa.walletaccountant.adapter.out.readmodel.ledger.mapper

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerTransactionDocument
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import org.springframework.stereotype.Service

@Service
object LedgerTransactionMapper {
    fun toDocument(model: LedgerTransactionModel): LedgerTransactionDocument =
        LedgerTransactionDocument(
            transactionId = model.transactionId,
            movementTypeId = model.movementTypeId,
            action = model.action,
            amount = model.amount,
            date = model.date,
            sourceAccountId = model.sourceAccountId,
            description = model.description,
            notes = model.notes,
            tagIds = model.tagIds,
        )

    fun toModel(document: LedgerTransactionDocument): LedgerTransactionModel =
        LedgerTransactionModel(
            transactionId = document.transactionId,
            movementTypeId = document.movementTypeId,
            action = document.action,
            amount = document.amount,
            date = document.date,
            sourceAccountId = document.sourceAccountId,
            description = document.description,
            notes = document.notes,
            tagIds = document.tagIds,
        )
}
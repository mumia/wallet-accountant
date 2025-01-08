package com.wa.walletaccountant.adapter.out.readmodel.movementtype.mapper

import com.wa.walletaccountant.adapter.out.readmodel.movementtype.document.MovementTypeDocument
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import org.springframework.stereotype.Service

@Service
object MovementTypeMapper {
    fun toDocument(model: MovementTypeModel): MovementTypeDocument =
        MovementTypeDocument(
            movementTypeId = model.movementTypeId,
            movementAction = model.movementAction,
            accountId = model.accountId,
            sourceAccountId = model.sourceAccountId,
            description = model.description,
            notes = model.notes,
            tagIds = model.tagIds
        )

    fun toModel(document: MovementTypeDocument): MovementTypeModel =
        MovementTypeModel(
            movementTypeId = document.movementTypeId,
            movementAction = document.movementAction,
            accountId = document.accountId,
            sourceAccountId = document.sourceAccountId,
            description = document.description,
            notes = document.notes,
            tagIds = document.tagIds,
        )
}

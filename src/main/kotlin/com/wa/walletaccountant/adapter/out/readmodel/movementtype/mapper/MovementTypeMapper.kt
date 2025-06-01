package com.wa.walletaccountant.adapter.out.readmodel.movementtype.mapper

import com.wa.walletaccountant.adapter.out.readmodel.movementtype.document.MovementTypeDocument
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import org.springframework.stereotype.Service

@Service
object MovementTypeMapper {
    fun toDocument(model: MovementTypeModel): MovementTypeDocument =
        MovementTypeDocument(
            aggregateId = model.movementTypeId.toString(),
            movementAction = model.movementAction,
            accountId = model.accountId,
            sourceAccountId = model.sourceAccountId,
            description = model.description,
            notes = model.notes,
            tags = model.tagIds
        )

    fun toModel(document: MovementTypeDocument): MovementTypeModel =
        MovementTypeModel(
            movementTypeId = MovementTypeId.fromString(document.aggregateId),
            movementAction = document.movementAction,
            accountId = document.accountId,
            sourceAccountId = document.sourceAccountId,
            description = document.description,
            notes = document.notes,
            tagIds = document.tags,
        )
}

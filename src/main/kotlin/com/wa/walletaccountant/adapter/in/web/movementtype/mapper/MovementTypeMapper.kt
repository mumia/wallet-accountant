package com.wa.walletaccountant.adapter.`in`.web.movementtype.mapper

import com.wa.walletaccountant.adapter.`in`.web.movementtype.request.RegisterMovementTypeRequest
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.command.RegisterNewMovementTypeCommand
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

object MovementTypeMapper {
    fun toCommand(
        movementTypeId: MovementTypeId,
        request: RegisterMovementTypeRequest
    ): RegisterNewMovementTypeCommand {
        return RegisterNewMovementTypeCommand(
            movementTypeId = movementTypeId,
            movementAction = MovementAction.valueOf(request.action),
            accountId = AccountId.fromString(request.accountId),
            sourceAccountId = if (request.sourceAccountId != null) {
                AccountId.fromString(request.sourceAccountId)
            } else {
                null
            },
            description = request.description,
            notes = request.notes,
            tagIds = request.tagIds.map { TagId.fromString(it) }.toSet(),
        )
    }
}

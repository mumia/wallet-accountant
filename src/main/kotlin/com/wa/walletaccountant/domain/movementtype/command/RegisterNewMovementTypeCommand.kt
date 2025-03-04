package com.wa.walletaccountant.domain.movementtype.command

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class RegisterNewMovementTypeCommand(
    @TargetAggregateIdentifier
    val movementTypeId: MovementTypeId,
    val movementAction: MovementAction,
    val accountId: AccountId,
    var sourceAccountId: AccountId?,
    val description: String,
    var notes: String?,
    val tagIds: Set<TagId>,
)

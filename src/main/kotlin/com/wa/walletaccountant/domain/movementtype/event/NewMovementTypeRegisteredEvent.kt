package com.wa.walletaccountant.domain.movementtype.event

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class NewMovementTypeRegisteredEvent(
    val movementTypeId: MovementTypeId,
    val movementAction: MovementAction,
    val accountId: AccountId,
    var sourceAccountId: AccountId?,
    val description: String,
    var notes: String?,
    val tagIds: Set<TagId>,
)

package com.wa.walletaccountant.application.model.movementtype

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class MovementTypeModel(
    val movementTypeId: MovementTypeId,
    val movementAction: MovementAction,
    val accountId: AccountId,
    val sourceAccountId: AccountId?,
    val description: String,
    val notes: String?,
    val tagIds: Set<TagId>
)

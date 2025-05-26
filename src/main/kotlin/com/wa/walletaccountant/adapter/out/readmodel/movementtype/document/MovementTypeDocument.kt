package com.wa.walletaccountant.adapter.out.readmodel.movementtype.document

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.data.annotation.TypeAlias
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.data.mongodb.core.mapping.MongoId

@Document("movementType")
@TypeAlias("MovementType")
data class MovementTypeDocument(
    @MongoId
    val movementTypeId: MovementTypeId,
    val movementAction: MovementAction,
    val accountId: AccountId,
    val sourceAccountId: AccountId?,
    val description: String,
    val notes: String?,
    val tags: Set<TagId>
)

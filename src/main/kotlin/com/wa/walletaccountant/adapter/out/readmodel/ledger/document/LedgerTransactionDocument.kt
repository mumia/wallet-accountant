package com.wa.walletaccountant.adapter.out.readmodel.ledger.document

import com.wa.walletaccountant.domain.common.DateTime
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.data.mongodb.core.mapping.MongoId

data class LedgerTransactionDocument(
    @MongoId
    val transactionId: TransactionId,
    val movementTypeId: MovementTypeId?,
    val action: MovementAction,
    val amount: Money,
    val date: DateTime,
    val sourceAccountId: AccountId?,
    val description: String,
    val notes: String?,
    val tagIds: Set<TagId>
)

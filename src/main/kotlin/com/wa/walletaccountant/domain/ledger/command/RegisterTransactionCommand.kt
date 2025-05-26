package com.wa.walletaccountant.domain.ledger.command

import com.wa.walletaccountant.domain.common.DateTime
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class RegisterTransactionCommand(
    @TargetAggregateIdentifier
    val ledgerId: LedgerId,
    val transactionId: TransactionId,
    val movementTypeId: MovementTypeId?,
    val amount: Money,
    val date: DateTime,
    val sourceAccountId: AccountId?,
    val description: String,
    val notes: String?,
    val tagIds: Set<TagId>
)

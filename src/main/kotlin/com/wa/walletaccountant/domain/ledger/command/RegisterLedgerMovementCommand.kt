package com.wa.walletaccountant.domain.ledger.command

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.MovementId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class RegisterLedgerMovementCommand(
    @TargetAggregateIdentifier
    val ledgerId: LedgerId,
    val movementId: MovementId,
    val movementTypeId: MovementTypeId,
    val action: MovementAction,
    val amount: Money,
    val date: Date,
    val sourceAccountId: AccountId?,
    val description: String,
    val notes: String?,
    val tagIds: Set<TagId>
)

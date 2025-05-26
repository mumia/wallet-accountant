package com.wa.walletaccountant.adapter.`in`.web.ledger.mapper

import com.wa.walletaccountant.adapter.`in`.web.ledger.request.CloseBalanceForMonthRequest
import com.wa.walletaccountant.adapter.`in`.web.ledger.request.RegisterTransactionRequest
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.DateTime
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.CloseBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.command.RegisterTransactionCommand
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import java.time.Month
import java.time.Year

object LedgerMapper {
    fun toCommand(transactionId: TransactionId, request: RegisterTransactionRequest): RegisterTransactionCommand {
        val date = DateTime.fromString(request.date)

        return RegisterTransactionCommand(
            ledgerId = LedgerId(
                accountId = AccountId.fromString(request.accountId),
                month = date.month(),
                year = date.year(),
            ),
            transactionId = transactionId,
            movementTypeId = request.transactionTypeId?.let { MovementTypeId.fromString(it) },
            amount = Money(request.amount),
            date = date,
            sourceAccountId = request.sourceAccountId?.let { AccountId.fromString(it) },
            description = request.description,
            notes = request.notes,
            tagIds = request.tagIds.map { TagId.fromString(it) }.toSet(),
        )
    }

    fun toCloseBalanceCommand(request: CloseBalanceForMonthRequest): CloseBalanceForMonthCommand {
        return CloseBalanceForMonthCommand(
            ledgerId = LedgerId(
                accountId = AccountId.fromString(request.accountId),
                month = Month.valueOf(request.month),
                year = Year.of(request.year),
            ),
            endBalance = Money(request.endBalance),
        )
    }
}
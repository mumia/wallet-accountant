package com.wa.walletaccountant.application.model.ledger

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.DateTime
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import java.time.Month
import java.time.Year

class LedgerMonthModelTest {
    private val accountId = "fb6d28c6-6431-4800-852b-7f9147f9893b"
    private val transactionId = "63f72822-c551-4ef4-ba51-3fdc5829a3f1"
    private val tagId = "c7d98af6-bb58-42bf-a822-5a1f2991f293"

    private val month = Month.JULY
    private val year = Year.of(2024)
    private val balance = Money(amount = 10000)
    private val amount = Money(amount = 1000)
    private val date = "2025-02-01T01:02:03.456Z"

    @Test
    fun shouldBeTheSame() {
        val model1 = LedgerMonthModel(
            ledgerId = LedgerId(AccountId.fromString(accountId), month, year),
            accountId = AccountId.fromString(accountId),
            month = month,
            year = year,
            initialBalance = balance,
            balance = balance,
            transactions = setOf(
                LedgerTransactionModel(
                    transactionId = TransactionId.fromString(transactionId),
                    movementTypeId = null,
                    action = Debit,
                    amount = amount,
                    date = DateTime.fromString(date),
                    sourceAccountId = null,
                    description = "a description",
                    notes = "No notes",
                    tagIds = setOf(TagId.fromString(tagId)),
                )
            ),
            closed = false,
        )

        val model2 = LedgerMonthModel(
            ledgerId = LedgerId(AccountId.fromString(accountId), month, year),
            accountId = AccountId.fromString(accountId),
            month = month,
            year = year,
            initialBalance = balance,
            balance = balance,
            transactions = setOf(
                LedgerTransactionModel(
                    transactionId = TransactionId.fromString(transactionId),
                    movementTypeId = null,
                    action = Debit,
                    amount = amount,
                    date = DateTime.fromString(date),
                    sourceAccountId = null,
                    description = "a description",
                    notes = "No notes",
                    tagIds = setOf(TagId.fromString(tagId)),
                )
            ),
            closed = false,
        )

        assertEquals(model1, model2)
    }
}

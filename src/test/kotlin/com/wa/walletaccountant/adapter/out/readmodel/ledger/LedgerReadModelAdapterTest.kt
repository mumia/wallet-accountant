package com.wa.walletaccountant.adapter.out.readmodel.ledger

import com.wa.walletaccountant.adapter.out.readmodel.LedgerReadModelAdapter
import com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.LedgerRepository
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.DateTime
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Credit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.test.context.ActiveProfiles
import org.springframework.test.context.DynamicPropertyRegistry
import org.springframework.test.context.DynamicPropertySource
import org.testcontainers.containers.MongoDBContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers
import java.time.Month
import java.time.Year
import kotlin.test.assertEquals

@Testcontainers
@SpringBootTest()
@ActiveProfiles("testing")
class LedgerReadModelAdapterTest
@Autowired
constructor(
    val ledgerRepository: LedgerRepository,
    val ledgerReadModelAdapter: LedgerReadModelAdapter,
) {
    companion object {
        @Container
        @JvmField
        val mongoDBContainer: MongoDBContainer = MongoDBContainer("mongo:7.0.7")

        init {
            mongoDBContainer.start()
        }

        @DynamicPropertySource
        @JvmStatic
        fun setProperties(registry: DynamicPropertyRegistry) {
            registry.add("spring.data.mongodb.uri") { mongoDBContainer.replicaSetUrl }
        }

        private val month = Month.of(2)
        private val year = Year.of(2025)
        private val balance = Money(amount = 1000)
        private val accountId1 = AccountId.fromString("fb6d28c6-6431-4800-852b-7f9147f9893b")
        private val accountId2 = AccountId.fromString("63f72822-c551-4ef4-ba51-3fdc5829a3f1")
        private val ledgerId1 = LedgerId(accountId1, month, year)
        private val ledgerId2 = LedgerId(accountId2, month, year)

        private val transactionId1 = TransactionId.fromString("c7d98af6-bb58-42bf-a822-5a1f2991f293")
        private val transactionId2 = TransactionId.fromString("fde4b60f-9ec7-4a7d-b969-85d4a8dddfb2")

        private val tagId1 = TagId.fromString("6041ef1a-7957-4680-88d4-1f350283357d")
        private val tagId2 = TagId.fromString("a228ea35-4976-4702-8296-da355f649364")
    }

    @AfterEach
    fun cleanUp() {
        ledgerRepository.deleteAll()
    }

    @Test
    fun testOpenMonthBalanceAndReadLedgerMonth() {
        val resultEmpty1 = ledgerReadModelAdapter.readLedgerMonth(ledgerId1)
        var resultEmpty2 = ledgerReadModelAdapter.readLedgerMonth(ledgerId2)

        assertTrue(resultEmpty1.isEmpty)
        assertTrue(resultEmpty2.isEmpty)

        ledgerReadModelAdapter.openMonthBalance(ledgerId1, balance)

        val result1 = ledgerReadModelAdapter.readLedgerMonth(ledgerId1)
        resultEmpty2 = ledgerReadModelAdapter.readLedgerMonth(ledgerId2)

        assertFalse(result1.isEmpty)
        assertEquals(
            LedgerMonthModel(
                ledgerId = ledgerId1,
                accountId = ledgerId1.accountId,
                month = ledgerId1.month,
                year = ledgerId1.year,
                initialBalance = balance,
                balance = balance,
                transactions = emptySet(),
                closed = false,
            ),
            result1.get(),
        )
        assertTrue(resultEmpty2.isEmpty)
    }

    @Test
    fun testCloseMonthBalance() {
        ledgerReadModelAdapter.openMonthBalance(ledgerId1, balance)

        var result1 = ledgerReadModelAdapter.readLedgerMonth(ledgerId1)
        assertEquals(
            LedgerMonthModel(
                ledgerId = ledgerId1,
                accountId = ledgerId1.accountId,
                month = ledgerId1.month,
                year = ledgerId1.year,
                initialBalance = balance,
                balance = balance,
                transactions = emptySet(),
                closed = false,
            ),
            result1.get(),
        )

        val closeResult = ledgerReadModelAdapter.closeMonthBalance(ledgerId1, balance)
        assertTrue(closeResult)

        result1 = ledgerReadModelAdapter.readLedgerMonth(ledgerId1)
        assertEquals(
            LedgerMonthModel(
                ledgerId = ledgerId1,
                accountId = ledgerId1.accountId,
                month = ledgerId1.month,
                year = ledgerId1.year,
                initialBalance = balance,
                balance = balance,
                transactions = emptySet(),
                closed = true,
            ),
            result1.get(),
        )
    }

    @Test
    fun testRegisterTransaction() {
        val date = DateTime.fromString("2025-01-30T01:02:03.456Z")

        val transaction1 = LedgerTransactionModel(
            transactionId = transactionId1,
            null,
            Debit,
            Money(10),
            date = date,
            null,
            "a transaction",
            null,
            HashSet(setOf(tagId1))
        )

        val transaction2 = LedgerTransactionModel(
            transactionId = transactionId2,
            null,
            Credit,
            Money(amount = 20),
            date = date,
            null,
            "another transaction",
            null,
            HashSet(setOf(tagId2))
        )

        val ledgerNoTransactions = LedgerMonthModel(
            ledgerId = ledgerId1,
            accountId = ledgerId1.accountId,
            month = ledgerId1.month,
            year = ledgerId1.year,
            initialBalance = balance,
            balance = balance,
            transactions = setOf(),
            closed = false,
        )

        val ledgerFirstTransactions = LedgerMonthModel(
            ledgerId = ledgerId1,
            accountId = ledgerId1.accountId,
            month = ledgerId1.month,
            year = ledgerId1.year,
            initialBalance = balance,
            balance = balance.add(transaction1.amount),
            transactions = setOf(transaction1),
            closed = false,
        )

        val ledgerSecondTransaction = LedgerMonthModel(
            ledgerId = ledgerId1,
            accountId = ledgerId1.accountId,
            month = ledgerId1.month,
            year = ledgerId1.year,
            initialBalance = balance,
            balance = balance.add(transaction1.amount).add(transaction2.amount),
            transactions = setOf(transaction1, transaction2),
            closed = false,
        )

        
        ledgerReadModelAdapter.openMonthBalance(ledgerId1, balance)

        assertEquals(ledgerNoTransactions, ledgerReadModelAdapter.readLedgerMonth(ledgerId1).get())
        assertTrue(ledgerReadModelAdapter.registerTransaction(ledgerId1, transaction1))
        assertEquals(ledgerFirstTransactions, ledgerReadModelAdapter.readLedgerMonth(ledgerId1).get())
        assertTrue(ledgerReadModelAdapter.registerTransaction(ledgerId1, transaction2))
        assertEquals(ledgerSecondTransaction, ledgerReadModelAdapter.readLedgerMonth(ledgerId1).get())
    }
}

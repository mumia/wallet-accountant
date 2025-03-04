package com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.ledgerrepository

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerTransactionDocument
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import org.bson.types.Decimal128
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.data.mongodb.core.MongoTemplate
import org.springframework.data.mongodb.core.query.Criteria
import org.springframework.data.mongodb.core.query.Query
import org.springframework.data.mongodb.core.query.Update
import java.math.BigDecimal

class LedgerRepositoryCustomImpl
@Autowired
constructor(
    private val mongoTemplate: MongoTemplate,
) : LedgerRepositoryCustom {
    override fun endMonth(id: LedgerId, balance: Money): Boolean {
        return updateLedgerMonth(id, Update().set("balance", balance).set("closed", true))
    }

    override fun registerTransaction(id: LedgerId, transaction: LedgerTransactionDocument): Boolean {
        val amount: BigDecimal
        if (transaction.action == Debit) {
            amount = transaction.amount.toInvertedBigDecimal()
        } else {
            amount = transaction.amount.toBigDecimal()
        }

        val update = Update()
            .addToSet("transactions", transaction)
            .inc("balance.amount", Decimal128(amount))

        return updateLedgerMonth(id, update)
    }

    private fun updateLedgerMonth(id: LedgerId, update: Update): Boolean {
        val query = Query(Criteria.where("_id").`is`(id))

        val result = mongoTemplate.updateFirst(query, update, LedgerMonthDocument::class.java)

        return result.wasAcknowledged() && result.modifiedCount > 0
    }
}

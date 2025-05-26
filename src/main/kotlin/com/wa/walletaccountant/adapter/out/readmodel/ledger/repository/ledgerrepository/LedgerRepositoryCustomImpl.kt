package com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.ledgerrepository

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerTransactionDocument
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.data.mongodb.core.MongoTemplate
import org.springframework.data.mongodb.core.query.Criteria
import org.springframework.data.mongodb.core.query.Query
import org.springframework.data.mongodb.core.query.Update

class LedgerRepositoryCustomImpl
@Autowired
constructor(
    private val mongoTemplate: MongoTemplate,
) : LedgerRepositoryCustom {
    override fun endMonth(id: LedgerId, balance: Money): Boolean {
        return updateLedgerMonth(id, Update().set("balance", balance).set("closed", true))
    }

    override fun registerTransaction(id: LedgerId, transaction: LedgerTransactionDocument): Boolean {
        val update = Update().addToSet("transactions", transaction)

        return updateLedgerMonth(id, update)
    }

    private fun updateLedgerMonth(id: LedgerId, update: Update): Boolean {
        val query = Query(Criteria.where("_id").`is`(id))

        val result = mongoTemplate.updateFirst(query, update, LedgerMonthDocument::class.java)

        return result.wasAcknowledged() && result.modifiedCount > 0
    }
}

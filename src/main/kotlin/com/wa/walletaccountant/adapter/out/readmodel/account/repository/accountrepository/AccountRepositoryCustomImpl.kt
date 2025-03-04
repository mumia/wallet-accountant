package com.wa.walletaccountant.adapter.out.readmodel.account.repository.accountrepository

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Date
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.data.mongodb.core.MongoTemplate
import org.springframework.data.mongodb.core.query.Criteria
import org.springframework.data.mongodb.core.query.Query
import org.springframework.data.mongodb.core.query.Update
import org.springframework.stereotype.Component

@Component
class AccountRepositoryCustomImpl
@Autowired
constructor(
    private val mongoTemplate: MongoTemplate,
) : AccountRepositoryCustom {
    override fun updateCurrentMonth(id: AccountId, currentMonth: Date): Boolean {
        val query = Query(Criteria.where("_id").`is`(id))
        val update = Update().set("currentMonth", currentMonth)

        val result = mongoTemplate.updateFirst(query, update, LedgerMonthDocument::class.java)

        return result.wasAcknowledged() && result.modifiedCount > 0
    }
}

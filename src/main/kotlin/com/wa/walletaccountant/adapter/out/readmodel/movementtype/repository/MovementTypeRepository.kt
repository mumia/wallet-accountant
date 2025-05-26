package com.wa.walletaccountant.adapter.out.readmodel.movementtype.repository

import com.wa.walletaccountant.adapter.out.readmodel.movementtype.document.MovementTypeDocument
import com.wa.walletaccountant.domain.account.account.AccountId
import org.springframework.data.mongodb.repository.MongoRepository

interface MovementTypeRepository: MongoRepository<MovementTypeDocument, String> {
    fun findByAccountId(accountId: AccountId): List<MovementTypeDocument>
}

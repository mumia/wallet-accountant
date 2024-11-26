package com.wa.walletaccountant.adapter.out.readmodel.account.repository

import com.wa.walletaccountant.adapter.out.readmodel.account.document.AccountDocument
import com.wa.walletaccountant.domain.account.account.AccountId
import org.springframework.data.mongodb.repository.MongoRepository

interface AccountRepository : MongoRepository<AccountDocument, AccountId>

package com.wa.walletaccountant.adapter.out.readmodel.account.repository

import com.wa.walletaccountant.adapter.out.readmodel.account.document.AccountDocument
import com.wa.walletaccountant.adapter.out.readmodel.account.repository.accountrepository.AccountRepositoryCustom
import org.springframework.data.mongodb.repository.MongoRepository

interface AccountRepository : MongoRepository<AccountDocument, String>, AccountRepositoryCustom

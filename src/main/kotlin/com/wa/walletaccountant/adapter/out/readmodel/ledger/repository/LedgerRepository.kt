package com.wa.walletaccountant.adapter.out.readmodel.ledger.repository

import com.wa.walletaccountant.adapter.out.readmodel.ledger.document.LedgerMonthDocument
import com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.ledgerrepository.LedgerRepositoryCustom
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.data.mongodb.repository.MongoRepository

interface LedgerRepository: MongoRepository<LedgerMonthDocument, LedgerId>, LedgerRepositoryCustom

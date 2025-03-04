package com.wa.walletaccountant.adapter.out.readmodel

import com.wa.walletaccountant.adapter.out.readmodel.account.repository.AccountRepository
import com.wa.walletaccountant.adapter.out.readmodel.ledger.mapper.LedgerMonthMapper
import com.wa.walletaccountant.adapter.out.readmodel.ledger.mapper.LedgerTransactionMapper
import com.wa.walletaccountant.adapter.out.readmodel.ledger.repository.LedgerRepository
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.application.port.out.LedgerReadModelPort
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.springframework.stereotype.Component
import java.util.Optional

@Component
class LedgerReadModelAdapter(
    private val repository: LedgerRepository,
    private val accountRepository: AccountRepository
) : LedgerReadModelPort {
    override fun openMonthBalance(id: LedgerId, balance: Money) {
        repository.save(LedgerMonthMapper.toDocument(id, balance))
    }

    override fun closeMonthBalance(id: LedgerId, balance: Money): Boolean {
        return repository.endMonth(id, balance)
    }

    override fun registerTransaction(id: LedgerId, transactionModel: LedgerTransactionModel): Boolean {
        return repository.registerTransaction(id, LedgerTransactionMapper.toDocument(transactionModel))
    }

    override fun readCurrentMonthLedger(accountId: AccountId): Optional<LedgerMonthModel> {
        val optionalAccount = accountRepository.findById(accountId)

        if (optionalAccount.isEmpty) {
            return Optional.empty()
        }

        val account = optionalAccount.get()
        val ledgerId = LedgerId(
            accountId = accountId,
            month = account.currentMonth.month(),
            year = account.currentMonth.year(),
        )

        val optionalLedgerMonth = repository.findById(ledgerId)

        if (optionalLedgerMonth.isEmpty) {
            return Optional.empty()
        }

        return Optional.of(LedgerMonthMapper.toModel(optionalLedgerMonth.get()))
    }

    override fun readLedgerMonth(id: LedgerId): Optional<LedgerMonthModel> {
        val optionalLedger = repository.findById(id);

        if (optionalLedger.isEmpty) {
            return Optional.empty<LedgerMonthModel>()
        }

        return Optional.of(LedgerMonthMapper.toModel(optionalLedger.get()))
    }
}
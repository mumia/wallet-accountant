package com.wa.walletaccountant.adapter.out.readmodel

import com.wa.walletaccountant.adapter.out.readmodel.account.mapper.AccountMapper
import com.wa.walletaccountant.adapter.out.readmodel.account.repository.AccountRepository
import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.port.out.AccountReadModelPort
import com.wa.walletaccountant.domain.account.account.AccountId
import org.springframework.stereotype.Service
import java.util.Optional

@Service
class AccountReadModelAdapter(
    private val accountMapper: AccountMapper,
    private val accountRepository: AccountRepository,
) : AccountReadModelPort {
    override fun registerNewAccount(model: AccountModel) {
        accountRepository.save(accountMapper.toDocument(model))
    }

    override fun readAccount(id: AccountId): Optional<AccountModel> {
        val optionalAccount = accountRepository.findById(id)
        if (optionalAccount.isEmpty) {
            return Optional.empty()
        }

        return Optional.of(accountMapper.toModel(optionalAccount.get()))
    }

    override fun readAllAccounts(): Set<AccountModel> =
        accountRepository
            .findAll()
            .map { accountMapper.toModel(it) }
            .toSet()
}

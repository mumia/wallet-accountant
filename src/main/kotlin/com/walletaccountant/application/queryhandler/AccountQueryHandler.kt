package com.walletaccountant.application.queryhandler

import com.walletaccountant.application.port.out.AccountReadModelRepository
import com.walletaccountant.application.readmodel.account.AccountReadModel
import com.walletaccountant.domain.account.query.GetAccountByIdQuery
import com.walletaccountant.domain.account.query.GetAllAccountsQuery
import org.axonframework.messaging.queryhandling.annotation.QueryHandler
import org.springframework.stereotype.Component

@Component
class AccountQueryHandler(private val repository: AccountReadModelRepository) {

    @QueryHandler
    fun handle(query: GetAccountByIdQuery): AccountReadModel? {
        return repository.findByAccountId(query.accountId.toJsonString())
    }

    @QueryHandler
    fun handle(query: GetAllAccountsQuery): List<AccountReadModel> {
        return repository.findAll()
    }
}

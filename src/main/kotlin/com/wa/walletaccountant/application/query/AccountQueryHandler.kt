package com.wa.walletaccountant.application.query

import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.port.out.AccountReadModelPort
import com.wa.walletaccountant.application.query.account.ReadAccountById
import com.wa.walletaccountant.application.query.account.ReadAllAccounts
import org.axonframework.queryhandling.QueryHandler
import org.springframework.stereotype.Component
import java.util.Optional

@Component
class AccountQueryHandler(
    private val readModel: AccountReadModelPort,
) {
    @QueryHandler
    fun readAccountById(query: ReadAccountById): Optional<AccountModel> = readModel.readAccount(query.accountId)

    @QueryHandler
    fun readAllAccounts(query: ReadAllAccounts): Set<AccountModel> = readModel.readAllAccounts()
}

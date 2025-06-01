package com.wa.walletaccountant.application.port.out

import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.model.account.AccountModel.ActiveMonth
import com.wa.walletaccountant.domain.account.account.AccountId
import java.util.Optional

interface AccountReadModelPort {
    fun registerNewAccount(model: AccountModel)

    fun updateActiveMonth(id: AccountId, activeMonth: ActiveMonth): Boolean

    fun readAccount(id: AccountId): Optional<AccountModel>

    fun readAllAccounts(): Set<AccountModel>

    fun accountExistsById(accountId: AccountId): Boolean
}

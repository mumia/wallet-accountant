package com.wa.walletaccountant.application.interceptor.service.account

import com.wa.walletaccountant.application.interceptor.exception.UnknownAccountException
import com.wa.walletaccountant.application.port.out.AccountReadModelPort
import com.wa.walletaccountant.domain.account.account.AccountId
import org.springframework.stereotype.Component

@Component
class AccountValidator(
    private val readModel: AccountReadModelPort,
) {
    fun validateAccountExists(accountId: AccountId) {
        if (!readModel.accountExistsById(accountId)) {
            throw UnknownAccountException.fromAccountId(accountId)
        }
    }
}

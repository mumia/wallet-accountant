package com.wa.walletaccountant.application.query.account

import com.wa.walletaccountant.domain.account.account.AccountId

data class ReadAccountById(
    val accountId: AccountId,
) {
    override fun toString(): String = accountId.toString()
}

package com.wa.walletaccountant.application.interceptor.exception

import com.wa.walletaccountant.domain.account.account.AccountId

class UnknownAccountException private constructor(
    identifier: String,
) : UnknownEntityException(
    "Unknown account. [%s]".format(identifier),
) {
    companion object {
        fun fromAccountId(accountId: AccountId): UnknownAccountException =
            UnknownAccountException(
                "AccountId: %s".format(accountId.toString()),
            )
    }
}

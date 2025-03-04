package com.wa.walletaccountant.application.query.ledger

import com.wa.walletaccountant.domain.account.account.AccountId

data class ReadCurrentMonthLedger(val accountId: AccountId) {
    override fun toString(): String = accountId.toString()
}

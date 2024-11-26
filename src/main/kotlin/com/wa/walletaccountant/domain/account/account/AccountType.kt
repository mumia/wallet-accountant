package com.wa.walletaccountant.domain.account.account

enum class AccountType {
    CHECKING {
        override fun fullName(): String = "Checking"
    },
    SAVINGS {
        override fun fullName(): String = "Savings"
    }, ;

    abstract fun fullName(): String
}

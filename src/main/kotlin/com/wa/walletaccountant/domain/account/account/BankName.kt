package com.wa.walletaccountant.domain.account.account

enum class BankName {
    DB {
        override fun fullName(): String = "Deutsche Bank"
    },
    N26 {
        override fun fullName(): String = "N26"
    },
    BCP {
        override fun fullName(): String = "Millennium bcp"
    }, ;

    abstract fun fullName(): String
}

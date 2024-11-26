package com.wa.walletaccountant.domain.common

enum class Currency {
    EUR {
        override fun fullName(): String = "Euro"
    },
    USD {
        override fun fullName(): String = "US Dollar"
    },
    CHF {
        override fun fullName(): String = "Swiss franc"
    }, ;

    abstract fun fullName(): String
}

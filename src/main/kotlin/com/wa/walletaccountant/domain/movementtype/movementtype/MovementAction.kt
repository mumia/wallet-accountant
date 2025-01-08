package com.wa.walletaccountant.domain.movementtype.movementtype

enum class MovementAction {
    Debit {
        override fun fullName(): String = "Debit"
    },
    Credit {
        override fun fullName(): String = "Credit"
    }, ;

    abstract fun fullName(): String
}
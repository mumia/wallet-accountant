package com.wa.walletaccountant.domain.common

abstract class AggregateId {
    abstract fun id(): String
}
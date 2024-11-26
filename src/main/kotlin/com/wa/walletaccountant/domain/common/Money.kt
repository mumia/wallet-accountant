package com.wa.walletaccountant.domain.common

data class Money(
    val value: Double,
    val currency: Currency,
) {
    override fun toString(): String = "%.2f %s".format(value, currency.toString())
}

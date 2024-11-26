package com.wa.walletaccountant.domain.common

data class Money(
    val value: Double,
    val currency: Currency,
) {
    override fun toString(): String = "%.2f %s".format(value, currency.toString())

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as Money

        return value == other.value &&
            currency == other.currency
    }

    override fun hashCode(): Int {
        var result = value.hashCode()
        result = 31 * result + currency.hashCode()

        return result
    }
}

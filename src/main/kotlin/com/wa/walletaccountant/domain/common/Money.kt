package com.wa.walletaccountant.domain.common

import com.wa.walletaccountant.domain.common.exception.MismatchedMoneyCurrencyException
import java.math.BigDecimal

data class Money(
    val value: BigDecimal,
    val currency: Currency,
) {
    fun subtract(value: Money): Money {
        checkCurrencyEquality(value.currency)

        return Money(this.value.subtract(value.value), currency)
    }

    fun add(value: Money): Money {
        checkCurrencyEquality(value.currency)

        return Money(this.value.add(value.value), currency)
    }

    private fun checkCurrencyEquality(givenCurrency: Currency) {
        if (!this.currency.equals(givenCurrency)) {
            throw MismatchedMoneyCurrencyException(this.currency, givenCurrency)
        }
    }

    override fun toString(): String = "%s %s".format(value.toPlainString(), currency.toString())

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

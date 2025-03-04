package com.wa.walletaccountant.domain.common

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import com.wa.walletaccountant.common.serialization.MoneySerializer
import java.math.BigDecimal
import java.math.RoundingMode.HALF_UP

@JsonSerialize(using = MoneySerializer::class)
class Money(private var amount: BigDecimal) {
    constructor(amount: Int): this(BigDecimal(amount))
    constructor(amount: Long): this(BigDecimal(amount))
    @JsonCreator
    constructor(amount: Double): this(BigDecimal(amount))

    init {
        amount = amount.setScale(2, HALF_UP)
    }

    fun subtract(value: Money): Money {
        return Money(this.amount.subtract(value.amount).setScale(2, HALF_UP))
    }

    fun add(value: Money): Money {
        return Money(this.amount.add(value.amount).setScale(2, HALF_UP))
    }

    fun toBigDecimal(): BigDecimal {
        return amount
    }

    fun toInvertedBigDecimal(): BigDecimal {
        return amount * BigDecimal(-1)
    }

    fun isNegative(): Boolean {
        return amount < BigDecimal.ZERO
    }

    @JsonValue
    private fun toDouble(): Double = amount.toDouble()

    override fun toString(): String = this.amount.toPlainString()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as Money

        return this.amount == other.amount
    }

    override fun hashCode(): Int {
        val result = amount.hashCode()

        return result
    }
}

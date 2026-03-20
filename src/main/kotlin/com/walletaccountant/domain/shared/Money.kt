package com.walletaccountant.domain.shared

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import java.math.BigDecimal
import java.math.RoundingMode

class Money private constructor(@JsonValue val amount: BigDecimal) {

    companion object {
        private const val SCALE = 2

        @JvmStatic
        @JsonCreator
        fun of(amount: BigDecimal): Money {
            require(amount.scale() <= SCALE) {
                "Money amount must have at most $SCALE decimal places, but had ${amount.scale()}"
            }
            return Money(amount.setScale(SCALE, RoundingMode.UNNECESSARY))
        }
    }

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (other !is Money) return false
        return amount.compareTo(other.amount) == 0
    }

    override fun hashCode(): Int = amount.stripTrailingZeros().hashCode()

    override fun toString(): String = "Money(amount=$amount)"
}

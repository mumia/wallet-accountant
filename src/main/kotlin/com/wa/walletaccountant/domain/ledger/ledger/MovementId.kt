package com.wa.walletaccountant.domain.ledger.ledger

import com.fasterxml.jackson.annotation.JsonValue
import java.util.UUID

data class MovementId(
    val value: UUID,
) {
    companion object {
        fun fromString(stringValue: String) = MovementId(UUID.fromString(stringValue))
    }

    @JsonValue
    override fun toString(): String = value.toString()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as MovementId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

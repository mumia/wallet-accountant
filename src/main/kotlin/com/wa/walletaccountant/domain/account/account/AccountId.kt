package com.wa.walletaccountant.domain.account.account

import com.fasterxml.jackson.annotation.JsonValue
import java.util.UUID

data class AccountId(
    val value: UUID,
) {
    companion object {
        fun fromString(stringValue: String) = AccountId(UUID.fromString(stringValue))
    }

    @JsonValue
    override fun toString(): String = value.toString()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as AccountId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

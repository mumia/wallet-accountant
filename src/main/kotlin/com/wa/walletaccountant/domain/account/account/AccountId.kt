package com.wa.walletaccountant.domain.account.account

import com.fasterxml.jackson.annotation.JsonValue
import com.wa.walletaccountant.domain.common.AggregateId
import java.util.UUID

data class AccountId(
    val value: UUID,
): AggregateId() {
    companion object {
        fun fromString(stringValue: String) = AccountId(UUID.fromString(stringValue))
    }

    override fun id(): String = value.toString()

    @JsonValue
    override fun toString(): String = id()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as AccountId

        return value.toString() == other.value.toString()
    }

    override fun hashCode(): Int = value.hashCode()
}

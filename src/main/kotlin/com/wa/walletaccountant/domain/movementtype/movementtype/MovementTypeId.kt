package com.wa.walletaccountant.domain.movementtype.movementtype

import com.fasterxml.jackson.annotation.JsonValue
import com.wa.walletaccountant.domain.common.AggregateId
import java.util.UUID

data class MovementTypeId(val value: UUID) : AggregateId() {
    companion object {
        fun fromString(stringValue: String) = MovementTypeId(UUID.fromString(stringValue))

        fun fromUUID(uuid: UUID) = MovementTypeId(uuid)
    }

    override fun id(): String = value.toString()

    @JsonValue
    override fun toString(): String = id()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as MovementTypeId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

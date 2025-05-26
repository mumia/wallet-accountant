package com.wa.walletaccountant.domain.tagcategory.tagcategory.tag

import com.fasterxml.jackson.annotation.JsonValue
import com.wa.walletaccountant.domain.common.AggregateId
import java.util.UUID

data class TagId(val value: UUID): AggregateId() {
    companion object {
        fun fromString(stringValue: String) = TagId(UUID.fromString(stringValue))

        fun fromUUID(uuid: UUID) = TagId(uuid)
    }

    override fun id(): String = value.toString()

    @JsonValue
    override fun toString(): String = id()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as TagId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

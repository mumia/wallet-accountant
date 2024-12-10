package com.wa.walletaccountant.domain.tagcategory.tagcategory.tag

import com.fasterxml.jackson.annotation.JsonValue
import java.util.UUID

data class TagId(
    val value: UUID,
) {
    companion object {
        fun fromString(stringValue: String) = TagId(UUID.fromString(stringValue))

        fun fromUUID(uuid: UUID) = TagId(uuid)
    }

    @JsonValue
    override fun toString(): String = value.toString()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as TagId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

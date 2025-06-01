package com.wa.walletaccountant.domain.tagcategory.tagcategory

import com.fasterxml.jackson.annotation.JsonValue
import com.wa.walletaccountant.domain.common.AggregateId
import java.util.UUID

data class TagCategoryId(val value: UUID): AggregateId() {
    companion object {
        fun fromString(stringValue: String) = TagCategoryId(UUID.fromString(stringValue))

        fun fromUUID(uuid: UUID) = TagCategoryId(uuid)
    }

    override fun id(): String = value.toString()

    @JsonValue
    override fun toString(): String = id()

    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as TagCategoryId

        return value == other.value
    }

    override fun hashCode(): Int = value.hashCode()
}

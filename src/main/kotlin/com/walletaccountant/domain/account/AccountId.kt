package com.walletaccountant.domain.account

import com.fasterxml.jackson.annotation.JsonCreator
import com.fasterxml.jackson.annotation.JsonValue
import kotlin.uuid.Uuid

data class AccountId(val id: Uuid) {
    @JsonValue
    fun toJsonString(): String = id.toString()

    companion object {
        @JvmStatic
        @JsonCreator
        fun fromString(value: String): AccountId = AccountId(Uuid.parse(value))
    }
}

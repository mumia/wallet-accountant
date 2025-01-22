package com.wa.walletaccountant.domain.ledger.ledger

import com.wa.walletaccountant.domain.account.account.AccountId
import java.time.Month
import java.time.Year
import java.util.UUID

data class LedgerId(val accountId: AccountId, val month: Month, val year: Year) {
    override fun equals(other: Any?): Boolean {
        if (this === other) return true
        if (javaClass != other?.javaClass) return false

        other as LedgerId

        return accountId == other.accountId &&
                month == other.month &&
                year == other.year
    }

    fun id(): String {
        return idUUID().toString()
    }

    fun idUUID(): UUID {
        return UUID
            .nameUUIDFromBytes(
                "%s|%d|%d"
                    .format(accountId.toString(), month.value, year.value)
                    .toByteArray()
            )
    }

    override fun toString(): String {
        return id()
    }

    override fun hashCode(): Int {
        var result = accountId.hashCode()
        result = 31 * result + month.hashCode()
        result = 31 * result + year.hashCode()

        return result
    }
}

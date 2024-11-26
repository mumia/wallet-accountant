package com.wa.walletaccountant.domain.account.account

import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.api.Test

class AccountIdTest {
    @Test
    fun areTheSame() {
        val accountId1 = AccountId.fromString("67f51f27-1070-46d7-b651-98fd12038832")
        val accountId2 = AccountId.fromString("67f51f27-1070-46d7-b651-98fd12038832")

        assertEquals(accountId1, accountId2)
        assertEquals(accountId1.hashCode(), accountId2.hashCode())
    }

    @Test
    fun areNotTheSame() {
        val accountId1 = AccountId.fromString("67f51f27-1070-46d7-b651-98fd12038832")
        val accountId2 = AccountId.fromString("37f51f27-1070-46d7-b651-98fd12038831")

        assertNotEquals(accountId1, accountId2)
        assertNotEquals(accountId1.hashCode(), accountId2.hashCode())
    }
}

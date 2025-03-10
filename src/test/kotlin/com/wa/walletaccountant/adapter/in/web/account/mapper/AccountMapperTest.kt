package com.wa.walletaccountant.adapter.`in`.web.account.mapper

import com.wa.walletaccountant.adapter.`in`.web.account.request.NewAccountRequest
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType.CHECKING
import com.wa.walletaccountant.domain.account.account.BankName.DB
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.common.Currency.EUR
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import java.math.BigDecimal

class AccountMapperTest {
    @Test
    fun createsCommand() {
        val accountId = AccountId.fromString("c5be2bf8-4ffa-4b3e-a152-518cec206b1d")

        val request =
            NewAccountRequest(
                bankName = DB.toString(),
                name = "Bank name",
                accountType = CHECKING.toString(),
                startingBalance = BigDecimal.valueOf(12.34),
                startingBalanceDate = "2014-02-03",
                currency = EUR.toString(),
                notes = "A note",
            )

        val expectedCommand =
            RegisterNewAccountCommand(
                accountId = accountId,
                bankName = DB,
                name = request.name,
                accountType = CHECKING,
                startingBalance = Money(amount = 12.34),
                startingBalanceDate = Date.fromString("2014-02-03"),
                currency = EUR,
                notes = "A note",
            )

        val actualCommand = AccountMapper.toCommand(accountId, request)

        assertEquals(expectedCommand, actualCommand)
    }
}

package com.wa.walletaccountant.domain.account

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType.CHECKING
import com.wa.walletaccountant.domain.account.account.BankName.BCP
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.wa.walletaccountant.domain.common.Currency.EUR
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import org.axonframework.test.aggregate.AggregateTestFixture
import org.axonframework.test.aggregate.FixtureConfiguration
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class AccountAggregateTest {
    var fixture: FixtureConfiguration<AccountAggregate>? = null
    val date1 = Date.fromString("2014-01-02")
    val date2 = Date.fromString("2014-01-02")

    @BeforeEach
    fun setUp() {
        fixture = AggregateTestFixture(AccountAggregate::class.java)
    }

    @Test
    fun registerAccount() {
        val accountId = "c5be2bf8-4ffa-4b3e-a152-518cec206b1d"
        val name = "Bcp mumia"
        val balance = 13.00

        val command =
            RegisterNewAccountCommand(
                accountId = AccountId.fromString(accountId),
                bankName = BCP,
                name = name,
                accountType = CHECKING,
                startingBalance = Money(balance, EUR),
                startingBalanceDate = date1,
                currency = EUR,
                notes = "",
            )

        val event =
            NewAccountRegisteredEvent(
                accountId = AccountId.fromString(accountId),
                bankName = BCP,
                name = name,
                accountType = CHECKING,
                startingBalance = Money(balance, EUR),
                startingBalanceDate = date2,
                currency = EUR,
                notes = "",
            )

        fixture!!
            .givenNoPriorActivity()
            .`when`(command)
            .expectSuccessfulHandlerExecution()
            .expectEvents(event)
    }
}
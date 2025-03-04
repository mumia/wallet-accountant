package com.wa.walletaccountant.application.saga

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType.CHECKING
import com.wa.walletaccountant.domain.account.account.BankName.BCP
import com.wa.walletaccountant.domain.account.command.StartNextMonthCommand
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.wa.walletaccountant.domain.account.event.NextMonthStartedEvent
import com.wa.walletaccountant.domain.common.Currency.EUR
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.OpenBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceClosedEvent
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import io.mockk.MockKAnnotations
import io.mockk.every
import io.mockk.impl.annotations.MockK
import io.mockk.junit5.MockKExtension
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.test.saga.FixtureConfiguration
import org.axonframework.test.saga.SagaTestFixture
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith

@ExtendWith(MockKExtension::class)
class LedgerLifecycleSagaTest {
    private lateinit var fixture: FixtureConfiguration
    private val accountId = AccountId.fromString("c5be2bf8-4ffa-4b3e-a152-518cec206b1d")
    private val name = "Bcp mumia"
    private val balance = Money(amount = 13.00)
    private val date = Date.fromString("2014-01-02")
    private val ledgerId = LedgerId(
        accountId = accountId,
        month = date.month(),
        year = date.year(),
    )
    private val nextLedgerId = LedgerId(
        accountId = accountId,
        month = date.month().plus(1),
        year = date.year(),
    )

    @MockK
    lateinit var commandGateway: CommandGateway

    @BeforeEach
    fun setUpEach() {
        MockKAnnotations.init(this)

        fixture = SagaTestFixture(LedgerLifecycleSaga::class.java)
        fixture.registerCommandGateway(CommandGateway::class.java, commandGateway)
    }

    @Test
    fun accountIsRegistered() {
        every { commandGateway.sendAndWait<LedgerId>(ofType(OpenBalanceForMonthCommand::class)) } returns ledgerId

        fixture
            .givenNoPriorActivity()
            .whenAggregate(accountId.toString())
            .publishes(
                NewAccountRegisteredEvent(
                    accountId = accountId,
                    bankName = BCP,
                    name = name,
                    accountType = CHECKING,
                    startingBalance = balance,
                    startingBalanceDate = date,
                    currency = EUR,
                    notes = "",
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectActiveSagas(0)
            .expectDispatchedCommands(
                OpenBalanceForMonthCommand(
                    ledgerId = ledgerId,
                    startBalance = balance,
                )
            )
    }

    @Test
    fun ledgerMonthBalanceClosed() {
        every { commandGateway.sendAndWait<LedgerId>(ofType(StartNextMonthCommand::class)) } returns ledgerId

        fixture
            .givenNoPriorActivity()
            .whenAggregate(ledgerId.toString())
            .publishes(
                MonthBalanceClosedEvent(
                    ledgerId = ledgerId,
                    closeBalance = balance
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectActiveSagas(1)
            .expectAssociationWith("accountId", accountId.toString())
            .expectDispatchedCommands(
                StartNextMonthCommand(
                    accountId = accountId,
                    balance = balance,
                )
            )
    }

    @Test
    fun accountStartsNewMonth() {
        every { commandGateway.sendAndWait<LedgerId>(ofType(StartNextMonthCommand::class)) } returns ledgerId
        every { commandGateway.sendAndWait<LedgerId>(ofType(OpenBalanceForMonthCommand::class)) } returns ledgerId

        fixture
            .givenAggregate(ledgerId.toString())
            .published(
                MonthBalanceClosedEvent(
                    ledgerId = ledgerId,
                    closeBalance = balance
                )
            )
            .whenAggregate(accountId.toString())
            .publishes(
                NextMonthStartedEvent(
                    accountId = accountId,
                    month = date.month().plus(1),
                    year = date.year(),
                    balance = balance
                )
            )
            .expectSuccessfulHandlerExecution()
            .expectActiveSagas(0)
            .expectDispatchedCommands(
                OpenBalanceForMonthCommand(
                    ledgerId = nextLedgerId,
                    startBalance = balance,
                )
            )
    }
}
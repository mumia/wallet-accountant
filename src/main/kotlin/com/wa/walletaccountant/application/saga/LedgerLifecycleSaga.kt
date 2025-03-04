package com.wa.walletaccountant.application.saga

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.command.StartNextMonthCommand
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.wa.walletaccountant.domain.account.event.NextMonthStartedEvent
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.OpenBalanceForMonthCommand
import com.wa.walletaccountant.domain.ledger.event.MonthBalanceClosedEvent
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import org.axonframework.modelling.saga.EndSaga
import org.axonframework.modelling.saga.SagaEventHandler
import org.axonframework.modelling.saga.SagaLifecycle
import org.axonframework.modelling.saga.StartSaga
import org.axonframework.spring.stereotype.Saga
import java.time.Month
import java.time.Year

@Saga
class LedgerLifecycleSaga : BaseSaga() {
    /**
     * Whenever a new account is registered, open a balance with the account starting month and balance
     */
    @StartSaga
    @EndSaga
    @SagaEventHandler(associationProperty = "accountId")
    fun on(event: NewAccountRegisteredEvent) {
        openBalanceForMonth(
            accountId = event.accountId,
            month = event.startingBalanceDate.month(),
            year = event.startingBalanceDate.year(),
            balance = event.startingBalance,
            eventName = event.javaClass.simpleName,
        )
    }

    /**
     * Whenever a ledger balance is closed, start the next month for the associated account
     */
    @StartSaga
    @SagaEventHandler(associationProperty = "ledgerId")
    fun on(event: MonthBalanceClosedEvent) {
        sendCommandAndWait(
            StartNextMonthCommand(
                accountId = event.ledgerId.accountId,
                balance = event.closeBalance,
            ),
            event.javaClass.simpleName
        )

        SagaLifecycle.associateWith("accountId", event.ledgerId.accountId.toString())
    }

    /**
     * Whenever an account starts the next month, open a balance with the account's current month and balance
     */
    @EndSaga
    @SagaEventHandler(associationProperty = "accountId")
    fun on(event: NextMonthStartedEvent) {
        openBalanceForMonth(
            accountId = event.accountId,
            month = event.month,
            year = event.year,
            balance = event.balance,
            eventName = event.javaClass.simpleName,
        )
    }

    private fun openBalanceForMonth(accountId: AccountId, month: Month, year: Year, balance: Money, eventName: String) {
        sendCommandAndWait(
            OpenBalanceForMonthCommand(
                ledgerId = LedgerId(
                    accountId = accountId,
                    month = month,
                    year = year,
                ),
                startBalance = balance,
            ),
            eventName
        )
    }
}

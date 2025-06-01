package com.wa.walletaccountant.application.projection.account

import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.model.account.AccountModel.ActiveMonth
import com.wa.walletaccountant.application.port.out.AccountReadModelPort
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.wa.walletaccountant.domain.account.event.NextMonthStartedEvent
import org.axonframework.config.ProcessingGroup
import org.axonframework.eventhandling.EventHandler
import org.springframework.stereotype.Component

@Component
@ProcessingGroup("account-read-model")
class AccountReadModelProjection(
    private val readModelPort: AccountReadModelPort,
) {
    @EventHandler
    fun on(event: NewAccountRegisteredEvent) {
        readModelPort.registerNewAccount(
            AccountModel(
                event.accountId.id(),
                event.accountId,
                event.bankName,
                event.name,
                event.accountType,
                event.startingBalance,
                event.startingBalanceDate,
                event.currency,
                event.notes,
                ActiveMonth(event.startingBalanceDate.month(), event.startingBalanceDate.year()),
            ),
        )
    }

    @EventHandler
    fun on(event: NextMonthStartedEvent) {
        val didUpdate = readModelPort.updateActiveMonth(
            id = event.accountId,
            activeMonth = ActiveMonth(event.month, event.year)
        )

        if (!didUpdate) {
            throw ActiveMonthUpdateFailureException(
                accountId = event.accountId,
                month = event.month,
                year = event.year
            )
        }
    }
}

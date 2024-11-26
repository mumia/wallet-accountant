package com.wa.walletaccountant.application.projection.account

import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.port.out.AccountReadModelPort
import com.wa.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import org.axonframework.config.ProcessingGroup
import org.axonframework.eventhandling.EventHandler
import org.springframework.stereotype.Component

@Component
@ProcessingGroup("read-models")
class AccountReadModelProjection(
    private val readModelPort: AccountReadModelPort,
) {
    @EventHandler
    fun on(event: NewAccountRegisteredEvent) {
        readModelPort.registerNewAccount(
            AccountModel(
                event.accountId,
                event.bankName,
                event.name,
                event.accountType,
                event.startingBalance,
                event.startingBalanceDate,
                event.currency,
                event.notes,
            ),
        )
    }
}

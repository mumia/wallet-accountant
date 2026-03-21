package com.walletaccountant.application.projection

import com.walletaccountant.application.port.out.AccountReadModelRepository
import com.walletaccountant.application.readmodel.account.AccountReadModel
import com.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import org.axonframework.messaging.eventhandling.annotation.EventHandler
import org.springframework.stereotype.Component

@Component
class AccountProjection(private val repository: AccountReadModelRepository) {

    @EventHandler
    fun on(event: NewAccountRegisteredEvent) {
        val readModel = AccountReadModel(
            accountId = event.accountId.toJsonString(),
            bankName = event.bankName,
            name = event.name,
            accountType = event.accountType,
            startingBalance = event.startingBalance,
            currency = event.currency,
            startingDate = event.startingDate.value,
            month = event.month,
            year = event.year,
            notes = event.notes
        )
        repository.save(readModel)
    }
}

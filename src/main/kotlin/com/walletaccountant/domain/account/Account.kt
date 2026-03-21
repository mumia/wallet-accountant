package com.walletaccountant.domain.account

import com.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.walletaccountant.domain.account.event.NewAccountRegisteredEvent
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.axonframework.eventsourcing.annotation.EventSourcedEntity
import org.axonframework.eventsourcing.annotation.EventSourcingHandler
import org.axonframework.eventsourcing.annotation.reflection.EntityCreator
import org.axonframework.messaging.commandhandling.annotation.CommandHandler
import org.axonframework.messaging.eventhandling.gateway.EventAppender
import java.time.Month
import java.time.Year

@EventSourcedEntity(tagKey = "accountId")
class Account {

    private lateinit var accountId: AccountId
    private lateinit var bankName: BankName
    private lateinit var name: String
    private lateinit var accountType: AccountType
    private lateinit var startingBalance: Money
    private lateinit var currency: Currency
    private lateinit var startingDate: Date
    private lateinit var month: Month
    private lateinit var year: Year
    private var notes: String? = null

    @EntityCreator
    constructor()

    companion object {
        @JvmStatic
        @CommandHandler
        fun handle(command: RegisterNewAccountCommand, eventAppender: EventAppender) {
            eventAppender.append(
                NewAccountRegisteredEvent(
                    accountId = command.accountId,
                    bankName = command.bankName,
                    name = command.name,
                    accountType = command.accountType,
                    startingBalance = command.startingBalance,
                    currency = command.currency,
                    startingDate = command.startingDate,
                    month = command.startingDate.month,
                    year = command.startingDate.year,
                    notes = command.notes
                )
            )
        }
    }

    @EventSourcingHandler
    fun on(event: NewAccountRegisteredEvent) {
        accountId = event.accountId
        bankName = event.bankName
        name = event.name
        accountType = event.accountType
        startingBalance = event.startingBalance
        currency = event.currency
        startingDate = event.startingDate
        month = event.month
        year = event.year
        notes = event.notes
    }
}

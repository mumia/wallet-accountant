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
import java.math.BigDecimal
import java.time.LocalDate
import java.time.Month
import java.time.Year
import kotlin.uuid.Uuid

@EventSourcedEntity(tagKey = "accountId")
class Account private constructor(
    private val accountId: AccountId,
    private val bankName: BankName,
    private val name: String,
    private val accountType: AccountType,
    private val startingBalance: Money,
    private val currency: Currency,
    private val startingDate: Date,
    private val month: Month,
    private val year: Year,
    private val notes: String?
) {

    @EntityCreator
    constructor() : this(
        accountId = AccountId(Uuid.NIL),
        bankName = BankName.entries.first(),
        name = "",
        accountType = AccountType.entries.first(),
        startingBalance = Money.of(BigDecimal.ZERO),
        currency = Currency.entries.first(),
        startingDate = Date(LocalDate.EPOCH),
        month = Month.JANUARY,
        year = Year.of(1970),
        notes = null
    )

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
    fun on(event: NewAccountRegisteredEvent): Account {
        return Account(
            accountId = event.accountId,
            bankName = event.bankName,
            name = event.name,
            accountType = event.accountType,
            startingBalance = event.startingBalance,
            currency = event.currency,
            startingDate = event.startingDate,
            month = event.month,
            year = event.year,
            notes = event.notes
        )
    }
}

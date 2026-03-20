package com.walletaccountant.application.interceptor

import com.walletaccountant.domain.account.AccountId
import com.walletaccountant.domain.account.AccountType
import com.walletaccountant.domain.account.BankName
import com.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.walletaccountant.domain.shared.Currency
import com.walletaccountant.domain.shared.Date
import com.walletaccountant.domain.shared.Money
import org.axonframework.messaging.commandhandling.CommandMessage
import org.axonframework.messaging.commandhandling.GenericCommandMessage
import org.axonframework.messaging.core.MessageHandlerInterceptorChain
import org.axonframework.messaging.core.MessageStream
import org.axonframework.messaging.core.MessageType
import org.axonframework.messaging.core.unitofwork.ProcessingContext
import org.axonframework.modelling.entity.EntityAlreadyExistsForCreationalCommandHandlerException
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import java.math.BigDecimal
import java.time.LocalDate
import kotlin.test.assertNotEquals
import kotlin.uuid.Uuid

class AggregateCreationRetryInterceptorTest {

    private lateinit var interceptor: AggregateCreationRetryInterceptor
    private lateinit var processingContext: ProcessingContext

    @BeforeEach
    fun setUp() {
        interceptor = AggregateCreationRetryInterceptor()
        processingContext = Mockito.mock(ProcessingContext::class.java)
    }

    @Test
    fun `should pass through command when no duplicate ID conflict`() {
        val command = createCommand()
        val message = GenericCommandMessage(MessageType(RegisterNewAccountCommand::class.java), command)
        var chainCalled = false

        val chain = MessageHandlerInterceptorChain<CommandMessage> { _, _ ->
            chainCalled = true
            MessageStream.fromItems<CommandMessage>()
        }

        interceptor.interceptOnHandle(message, processingContext, chain)

        assert(chainCalled) { "Chain should have been called" }
    }

    @Test
    fun `should retry with new ID on duplicate aggregate ID exception`() {
        val command = createCommand()
        val message = GenericCommandMessage(MessageType(RegisterNewAccountCommand::class.java), command)
        var retryCount = 0
        val receivedIds = mutableListOf<AccountId>()

        val chain = MessageHandlerInterceptorChain<CommandMessage> { msg, _ ->
            val payload = msg.payload()
            if (payload is RegisterNewAccountCommand) {
                receivedIds.add(payload.accountId)
            }
            retryCount++
            if (retryCount == 1) {
                throw EntityAlreadyExistsForCreationalCommandHandlerException(msg, "existing-entity")
            }
            MessageStream.fromItems<CommandMessage>()
        }

        interceptor.interceptOnHandle(message, processingContext, chain)

        assert(retryCount == 2) { "Expected 2 attempts but got $retryCount" }
        assert(receivedIds.size == 2) { "Expected 2 IDs but got ${receivedIds.size}" }
        assertNotEquals(receivedIds[0], receivedIds[1], "Retry should use a different ID")
    }

    @Test
    fun `should pass through non-HasAggregateId commands without retry`() {
        data class OtherCommand(val value: String)

        val message = GenericCommandMessage(MessageType(OtherCommand::class.java), OtherCommand("test"))
        var exceptionThrown = false

        val chain = MessageHandlerInterceptorChain<CommandMessage> { _, _ ->
            throw RuntimeException("Some error")
        }

        try {
            interceptor.interceptOnHandle(message, processingContext, chain)
        } catch (e: RuntimeException) {
            exceptionThrown = true
        }

        assert(exceptionThrown) { "Exception should propagate for non-HasAggregateId commands" }
    }

    @Test
    fun `should propagate non-duplicate exceptions for HasAggregateId commands`() {
        val command = createCommand()
        val message = GenericCommandMessage(MessageType(RegisterNewAccountCommand::class.java), command)
        var exceptionThrown = false

        val chain = MessageHandlerInterceptorChain<CommandMessage> { _, _ ->
            throw IllegalStateException("Not a duplicate ID error")
        }

        try {
            interceptor.interceptOnHandle(message, processingContext, chain)
        } catch (e: IllegalStateException) {
            exceptionThrown = true
        }

        assert(exceptionThrown) { "Non-duplicate exceptions should propagate" }
    }

    private fun createCommand(): RegisterNewAccountCommand =
        RegisterNewAccountCommand(
            accountId = AccountId(Uuid.parse("550e8400-e29b-41d4-a716-446655440000")),
            bankName = BankName.BCP,
            name = "Test Account",
            accountType = AccountType.CHECKING,
            startingBalance = Money.of(BigDecimal("100.00")),
            currency = Currency.EUR,
            startingDate = Date(LocalDate.of(2026, 1, 1)),
            notes = null
        )
}

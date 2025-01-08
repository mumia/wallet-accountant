package com.wa.walletaccountant.application.interceptor.command.movementtype

import com.wa.walletaccountant.application.interceptor.service.account.AccountValidator
import com.wa.walletaccountant.application.interceptor.service.tagcategory.TagValidator
import com.wa.walletaccountant.domain.movementtype.command.RegisterNewMovementTypeCommand
import org.axonframework.commandhandling.CommandMessage
import org.axonframework.messaging.MessageDispatchInterceptor
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.stereotype.Component
import java.util.function.BiFunction

@Component
class RegisterNewMovementTypeCommandInterceptor
@Autowired
constructor(
    private val accountValidator: AccountValidator,
    private val tagValidator: TagValidator,
) : MessageDispatchInterceptor<CommandMessage<*>> {
    override fun handle(messages: List<CommandMessage<*>>): BiFunction<Int, CommandMessage<*>, CommandMessage<*>> {
        return BiFunction<Int, CommandMessage<*>, CommandMessage<*>> { _: Int?, command: CommandMessage<*> ->
            val payload = command.payload as? RegisterNewMovementTypeCommand ?: return@BiFunction command

            accountValidator.validateAccountExists(payload.accountId)

            if (payload.sourceAccountId != null) {
                accountValidator.validateAccountExists(payload.sourceAccountId!!)
            }

            payload.tagIds.forEach { tagValidator.validateTagIdExists(it) }

            return@BiFunction command
        }
    }
}
package com.walletaccountant.application.interceptor

import org.axonframework.messaging.commandhandling.CommandMessage
import org.axonframework.messaging.commandhandling.GenericCommandMessage
import org.axonframework.messaging.core.MessageHandlerInterceptor
import org.axonframework.messaging.core.MessageHandlerInterceptorChain
import org.axonframework.messaging.core.MessageStream
import org.axonframework.messaging.core.MessageType
import org.axonframework.messaging.core.unitofwork.ProcessingContext
import org.axonframework.modelling.entity.EntityAlreadyExistsForCreationalCommandHandlerException
import org.springframework.stereotype.Component

@Component
class AggregateCreationRetryInterceptor : MessageHandlerInterceptor<CommandMessage> {

    override fun interceptOnHandle(
        message: CommandMessage,
        processingContext: ProcessingContext,
        chain: MessageHandlerInterceptorChain<CommandMessage>
    ): MessageStream<*> {
        val payload = message.payload()

        if (payload !is HasAggregateId<*>) {
            return chain.proceed(message, processingContext)
        }

        return try {
            chain.proceed(message, processingContext)
        } catch (e: EntityAlreadyExistsForCreationalCommandHandlerException) {
            val newPayload = (payload as HasAggregateId<*>).withNewId()
            val newMessage = GenericCommandMessage(MessageType(newPayload!!::class.java), newPayload)
            chain.proceed(newMessage, processingContext)
        }
    }
}

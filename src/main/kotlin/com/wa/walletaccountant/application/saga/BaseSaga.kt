package com.wa.walletaccountant.application.saga

import com.wa.walletaccountant.application.saga.exception.SagaEventHandlingTimedOut
import org.axonframework.commandhandling.gateway.CommandGateway
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.beans.factory.annotation.Autowired

abstract class BaseSaga {
    @Transient
    @Autowired
    protected lateinit var commandGateway: CommandGateway

    companion object {
        protected val log: Logger = LoggerFactory.getLogger(this::class.java)
    }

    fun <T : Any> sendCommandAndWait(command: T, eventName: String) {
        try {
            val result = commandGateway.sendAndWait<T>(command)

            if (result === null) {
                throw SagaEventHandlingTimedOut(this.javaClass.simpleName, eventName)
            }
        } catch (exception: SagaEventHandlingTimedOut) {
            log.warn("${exception.javaClass.simpleName}: ${exception.message}")

            throw exception
        } catch (exception: Throwable) {
            var cause: Throwable? = exception
            var previousCause: Throwable? = null
            var recursiveCount = 0

            do {
                if (!cause?.equals(previousCause)!!) {
                    log.error("${cause.javaClass.simpleName}: ${cause.message}")
                    recursiveCount = 0
                }

                previousCause = cause
                cause = exception.cause
                recursiveCount++
            } while (cause != null && recursiveCount < 10)

            throw exception
        }
    }
}
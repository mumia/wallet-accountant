package com.wa.walletaccountant.application.saga

import com.wa.walletaccountant.application.saga.exception.SagaEventHandlingTimedOut
import org.axonframework.commandhandling.gateway.CommandGateway
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.beans.factory.annotation.Autowired
import java.util.concurrent.TimeUnit.SECONDS
import java.util.concurrent.TimeoutException

abstract class BaseSaga {
    @Transient
    @Autowired
    protected lateinit var commandGateway: CommandGateway

    companion object {
        protected val log: Logger = LoggerFactory.getLogger(this::class.java)
    }

    fun <T : Any> sendCommandAndWait(command: T, eventName: String) {
        try {
            commandGateway.send<T>(command).get(5, SECONDS) // Throws TimeoutException
            // If we get here, we got a response (null or not)
        } catch (exception: TimeoutException) {
            handleTimeoutOrInterruption(exception, eventName)
        } catch (exception: InterruptedException) {
            handleTimeoutOrInterruption(exception, eventName)
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

    private fun handleTimeoutOrInterruption(exception: Exception, eventName: String) {
        val newException = SagaEventHandlingTimedOut(this.javaClass.simpleName, eventName, exception)

        log.warn("${newException.javaClass.simpleName}: ${newException.message}")

        throw newException
    }
}
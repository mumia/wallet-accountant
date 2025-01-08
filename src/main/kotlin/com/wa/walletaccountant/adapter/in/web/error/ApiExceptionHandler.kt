package com.wa.walletaccountant.adapter.`in`.web.error

import com.wa.walletaccountant.adapter.`in`.web.error.response.BadRequestResponse
import com.wa.walletaccountant.application.interceptor.exception.EntityExistenceException
import com.wa.walletaccountant.application.interceptor.exception.UnknownEntityException
import jakarta.validation.ConstraintViolationException
import org.axonframework.commandhandling.CommandExecutionException
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.http.HttpStatus.BAD_REQUEST
import org.springframework.http.ResponseEntity
import org.springframework.http.converter.HttpMessageNotReadableException
import org.springframework.web.bind.MethodArgumentNotValidException
import org.springframework.web.bind.annotation.ControllerAdvice
import org.springframework.web.bind.annotation.ExceptionHandler
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.method.annotation.MethodArgumentTypeMismatchException

@ControllerAdvice
class ApiExceptionHandler {
    companion object {
        private val log: Logger = LoggerFactory.getLogger(this::class.java)

        private val EXCEPTION_LOGGING_STR: String = "Handled exception: {}"
    }

    @ExceptionHandler(value = [MethodArgumentTypeMismatchException::class])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleMethodArgumentMismatchError(exception: MethodArgumentTypeMismatchException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        val badRequest =
            BadRequestResponse(
                title = "Invalid Argument Error",
                type = "Invalid Argument Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = exception.name,
                        reason = "Provided invalid value",
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }

    @ExceptionHandler(value = [ConstraintViolationException::class])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleConstraintViolationError(exception: ConstraintViolationException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        return ResponseEntity
            .badRequest()
            .body(ConstraintViolationResponseConverter.toConstraintViolationResponse(exception))
    }

    @ExceptionHandler(value = [MethodArgumentNotValidException::class])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleMethodArgumentNotValid(exception: MethodArgumentNotValidException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        return ResponseEntity
            .badRequest()
            .body(ConstraintViolationResponseConverter.toConstraintViolationResponse(exception))
    }

    @ExceptionHandler(value = [HttpMessageNotReadableException::class])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleError(exception: HttpMessageNotReadableException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        var cause: Exception = exception
        while (cause.cause != null) {
            cause = cause.cause as Exception
        }

        val badRequest =
            BadRequestResponse(
                title = "Invalid Argument Error",
                type = "Invalid Argument Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = cause.javaClass.simpleName,
                        reason = cause.message!!,
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }

    @ExceptionHandler(value = [EntityExistenceException::class, ])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleExistenceErrors(exception: EntityExistenceException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        val badRequest =
            BadRequestResponse(
                title = "Entity already exists",
                type = "Entity exists Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = exception.javaClass.simpleName,
                        reason = exception.message!!,
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }

    @ExceptionHandler(value = [UnknownEntityException::class, ])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleExistenceErrors(exception: UnknownEntityException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        val badRequest =
            BadRequestResponse(
                title = "Entity is unknown",
                type = "Unknown entity Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = exception.javaClass.simpleName,
                        reason = exception.message!!,
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }

    @ExceptionHandler(value = [CommandExecutionException::class])
    @ResponseStatus(value = BAD_REQUEST)
    fun handleExistenceErrors(exception: CommandExecutionException): ResponseEntity<Any> {
        log.warn(EXCEPTION_LOGGING_STR, exception.message)

        val badRequest =
            BadRequestResponse(
                title = "An aggregate found a logic error",
                type = "Aggregate logic Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = exception.javaClass.simpleName,
                        reason = exception.message!!,
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }
}

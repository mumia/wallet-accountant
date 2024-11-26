package com.wa.walletaccountant.adapter.`in`.web.error

import com.wa.walletaccountant.adapter.`in`.web.error.response.BadRequestResponse
import jakarta.validation.ConstraintViolationException
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
class ConstraintExceptionHandler {
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

        val badRequest =
            BadRequestResponse(
                title = "Invalid Argument Error",
                type = "Invalid Argument Exception",
                invalidParameters =
                    BadRequestResponse.InvalidParameters(
                        name = "Request Body is malformed",
                        reason = "Request body is not a valid json object",
                    ),
            )

        return ResponseEntity.badRequest().body(badRequest)
    }
}

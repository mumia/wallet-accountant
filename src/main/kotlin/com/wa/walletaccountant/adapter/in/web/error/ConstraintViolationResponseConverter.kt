package com.wa.walletaccountant.adapter.`in`.web.error

import com.wa.walletaccountant.adapter.`in`.web.error.ConstraintViolationResponse.Violation
import jakarta.validation.ConstraintViolation
import jakarta.validation.ConstraintViolationException
import org.springframework.web.bind.MethodArgumentNotValidException

object ConstraintViolationResponseConverter {
    fun toConstraintViolationResponse(exception: MethodArgumentNotValidException) =
        buildErrorHeader()
            .violations(
                exception
                    .bindingResult
                    .fieldErrors
                    .stream()
                    .map { it.unwrap(ConstraintViolation::class.java) }
                    .map { toConstraintViolation(it) }
                    .toList(),
            ).build()

    fun toConstraintViolationResponse(exception: ConstraintViolationException) =
        buildErrorHeader()
            .violations(
                exception
                    .constraintViolations
                    .stream()
                    .map { toConstraintViolation(it) }
                    .toList(),
            ).build()

    private fun buildErrorHeader() =
        ConstraintViolationResponse
            .Builder()
            .title("A validation constraint failed")
            .type("ConstraintViolation")

    private fun toConstraintViolation(constraintViolation: ConstraintViolation<*>) =
        Violation(
            name = constraintViolation.propertyPath.toString(),
            value =
                if (constraintViolation.invalidValue != null) {
                    constraintViolation.invalidValue.toString()
                } else {
                    "null"
                },
            reason = constraintViolation.message,
        )
}

package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.Constraint
import jakarta.validation.Payload
import kotlin.annotation.AnnotationRetention.RUNTIME
import kotlin.annotation.AnnotationTarget.FIELD
import kotlin.reflect.KClass

@Constraint(validatedBy = [DateDayFormatConstraintValidator::class])
@Target(FIELD)
@Retention(RUNTIME)
annotation class DateDayFormatConstraint(
    val message: String = "Expected date format is YYYY-MM-DD",
    val groups: Array<KClass<*>> = [],
    val payload: Array<KClass<out Payload>> = [],
)

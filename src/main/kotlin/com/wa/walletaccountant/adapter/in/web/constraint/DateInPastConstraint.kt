package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.Constraint
import jakarta.validation.Payload
import kotlin.annotation.AnnotationRetention.RUNTIME
import kotlin.annotation.AnnotationTarget.FIELD
import kotlin.reflect.KClass

@Constraint(validatedBy = [DateInPastConstraintValidator::class])
@Target(FIELD)
@Retention(RUNTIME)
annotation class DateInPastConstraint(
    val includeToday: Boolean = false,
    val message: String = "Date must be in the past",
    val groups: Array<KClass<*>> = [],
    val payload: Array<KClass<out Payload>> = [],
)

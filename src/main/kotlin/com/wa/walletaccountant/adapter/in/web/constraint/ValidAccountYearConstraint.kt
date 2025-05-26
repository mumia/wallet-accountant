package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.Constraint
import jakarta.validation.Payload
import kotlin.annotation.AnnotationRetention.RUNTIME
import kotlin.annotation.AnnotationTarget.FIELD
import kotlin.reflect.KClass

@Constraint(validatedBy = [ValidAccountYearConstraintValidator::class])
@Target(FIELD)
@Retention(RUNTIME)
annotation class ValidAccountYearConstraint(
    val message: String = "Year must be newer but not in the future",
    val groups: Array<KClass<*>> = [],
    val payload: Array<KClass<out Payload>> = [],
)

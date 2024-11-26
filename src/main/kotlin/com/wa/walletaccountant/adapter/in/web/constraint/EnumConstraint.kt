package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.Constraint
import jakarta.validation.Payload
import kotlin.annotation.AnnotationRetention.RUNTIME
import kotlin.annotation.AnnotationTarget.FIELD
import kotlin.reflect.KClass

@Constraint(validatedBy = [EnumConstraintValidator::class])
@Target(FIELD)
@Retention(RUNTIME)
annotation class EnumConstraint(
    val enumClass: KClass<out Enum<*>>,
    val message: String = "Unknown value for field, expected one of {values}",
    val groups: Array<KClass<*>> = [],
    val payload: Array<KClass<out Payload>> = [],
)

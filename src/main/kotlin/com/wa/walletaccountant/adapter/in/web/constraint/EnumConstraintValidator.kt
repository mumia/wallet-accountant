package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.ConstraintValidator
import jakarta.validation.ConstraintValidatorContext
import org.apache.commons.text.StringSubstitutor

class EnumConstraintValidator : ConstraintValidator<EnumConstraint, String> {
    private val acceptedValues: MutableList<String> = mutableListOf()
    private var customMessage: String? = null

    override fun initialize(constraintAnnotation: EnumConstraint?) {
        super.initialize(constraintAnnotation)

        acceptedValues.addAll(
            constraintAnnotation!!
                .enumClass.java.enumConstants
                .map { it.name },
        )
        customMessage = constraintAnnotation.message
    }

    override fun isValid(
        value: String?,
        context: ConstraintValidatorContext?,
    ): Boolean {
        if (value == null) {
            return true
        }

        if (acceptedValues.contains(value)) {
            return true
        }

        var message = context!!.defaultConstraintMessageTemplate
        if (customMessage != null) {
            message = customMessage
        }

        message =
            StringSubstitutor.replace(
                message,
                mapOf("values" to acceptedValues.joinToString(", ")),
                "{",
                "}",
            )

        context.disableDefaultConstraintViolation()
        context
            .buildConstraintViolationWithTemplate(message)
            .addConstraintViolation()

        return false
    }
}

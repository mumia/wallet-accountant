package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.ConstraintValidator
import jakarta.validation.ConstraintValidatorContext
import java.util.regex.Pattern

class DateInPastConstraintValidator : ConstraintValidator<DateInPastConstraint, String> {
    override fun isValid(
        value: String?,
        context: ConstraintValidatorContext?,
    ): Boolean {
        if (value == null) {
            return true
        }

        val pattern = Pattern.compile("([0-9]{4}-[0-9]{2}-[0-9]{2})")
        if (pattern.matcher(value).matches()) {
            return true
        }

        return false
    }
}

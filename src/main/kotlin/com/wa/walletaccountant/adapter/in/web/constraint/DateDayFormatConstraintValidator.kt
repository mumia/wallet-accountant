package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.ConstraintValidator
import jakarta.validation.ConstraintValidatorContext
import java.util.regex.Pattern

class DateDayFormatConstraintValidator : ConstraintValidator<DateDayFormatConstraint, String> {
    override fun isValid(
        value: String?,
        context: ConstraintValidatorContext?,
    ): Boolean {
        if (value == null) {
            return true
        }

        val pattern = Pattern.compile("([0-9]{4}-[0-9]{2}-[0-9]{2})(T00:00:00.000Z)?")
        if (pattern.matcher(value).matches()) {
            return true
        }

        return false
    }
}

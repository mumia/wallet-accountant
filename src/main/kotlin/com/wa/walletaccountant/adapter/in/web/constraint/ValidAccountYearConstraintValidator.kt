package com.wa.walletaccountant.adapter.`in`.web.constraint

import jakarta.validation.ConstraintValidator
import jakarta.validation.ConstraintValidatorContext
import java.time.Year

class ValidAccountYearConstraintValidator : ConstraintValidator<ValidAccountYearConstraint, Int> {
    companion object {
        private val MINIMUM_YEAR = 2000
    }

    override fun isValid(
        value: Int?,
        context: ConstraintValidatorContext?,
    ): Boolean {
        if (value == null) {
            return true
        }

        return value > MINIMUM_YEAR && value <= Year.now().value
    }
}

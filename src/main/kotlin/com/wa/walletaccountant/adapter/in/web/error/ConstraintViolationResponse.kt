package com.wa.walletaccountant.adapter.`in`.web.error

import com.fasterxml.jackson.annotation.JsonProperty

data class ConstraintViolationResponse(
    val type: String,
    val title: String,
    @JsonProperty("constraint-violations")
    val violations: List<Violation>,
) {
    private constructor(builder: Builder) : this(builder.type!!, builder.title!!, builder.violations!!)

    data class Violation(
        val name: String,
        val value: String,
        val reason: String,
    )

    class Builder {
        var type: String? = null
            private set
        var title: String? = null
            private set
        var violations: List<Violation>? = null
            private set

        fun type(type: String) = apply { this.type = type }

        fun title(title: String) = apply { this.title = title }

        fun violations(violations: List<Violation>) = apply { this.violations = violations }

        fun build() = ConstraintViolationResponse(this)
    }
}

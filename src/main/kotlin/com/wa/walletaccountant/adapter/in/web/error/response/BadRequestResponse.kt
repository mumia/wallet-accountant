package com.wa.walletaccountant.adapter.`in`.web.error.response

import com.fasterxml.jackson.annotation.JsonProperty

data class BadRequestResponse(
    val type: String,
    val title: String,
    @JsonProperty("invalid-params")
    val invalidParameters: InvalidParameters,
) {
    data class InvalidParameters(
        val name: String,
        val reason: String,
    )
}

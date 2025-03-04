package com.wa.walletaccountant.common.serialization

import com.fasterxml.jackson.core.JsonGenerator
import com.fasterxml.jackson.databind.JsonSerializer
import com.fasterxml.jackson.databind.SerializerProvider
import com.wa.walletaccountant.domain.common.Money

class MoneySerializer: JsonSerializer<Money>() {
    override fun serialize(value: Money?, jsonGenerator: JsonGenerator?, provider: SerializerProvider?) {
        jsonGenerator?.let { generator -> value?.let { _ -> generator.writeNumber(value.toString()) }}
    }
}
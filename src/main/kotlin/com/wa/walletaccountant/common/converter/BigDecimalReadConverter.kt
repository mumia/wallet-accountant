package com.wa.walletaccountant.common.converter

import org.bson.types.Decimal128
import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.math.BigDecimal

@Component
class BigDecimalReadConverter : Converter<Decimal128, BigDecimal> {
    override fun convert(value: Decimal128): BigDecimal = value.bigDecimalValue()
}

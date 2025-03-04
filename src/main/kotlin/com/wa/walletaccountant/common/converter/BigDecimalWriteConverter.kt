package com.wa.walletaccountant.common.converter

import org.bson.types.Decimal128
import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.math.BigDecimal

@Component
class BigDecimalWriteConverter : Converter<BigDecimal, Decimal128> {
    override fun convert(value: BigDecimal): Decimal128 = Decimal128(value)
}

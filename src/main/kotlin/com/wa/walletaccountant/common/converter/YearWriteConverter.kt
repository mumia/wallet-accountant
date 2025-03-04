package com.wa.walletaccountant.common.converter

import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.time.Year

@Component
class YearWriteConverter : Converter<Year, Int> {
    override fun convert(year: Year): Int = year.value
}

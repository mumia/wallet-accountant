package com.wa.walletaccountant.common.converter

import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.time.Year

@Component
class YearReadConverter : Converter<Int, Year> {
    override fun convert(year: Int): Year = Year.of(year)
}

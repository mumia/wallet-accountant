package com.wa.walletaccountant.common.converter

import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.time.ZoneId
import java.time.ZonedDateTime
import java.util.Date

@Component
class ZonedDateTimeReadConverter : Converter<Date, ZonedDateTime> {
    override fun convert(date: Date): ZonedDateTime = date.toInstant().atZone(ZoneId.of("UTC"))
}

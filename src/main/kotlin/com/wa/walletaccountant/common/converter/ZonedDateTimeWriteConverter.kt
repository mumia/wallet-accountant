package com.wa.walletaccountant.common.converter

import org.springframework.core.convert.converter.Converter
import org.springframework.stereotype.Component
import java.time.ZonedDateTime
import java.util.Date

@Component
class ZonedDateTimeWriteConverter : Converter<ZonedDateTime, Date> {
    override fun convert(zonedDateTime: ZonedDateTime): Date = Date.from(zonedDateTime.toInstant())
}

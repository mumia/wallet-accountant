package com.wa.walletaccountant.common.config

import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.core.convert.converter.Converter
import org.springframework.data.convert.ReadingConverter
import org.springframework.data.convert.WritingConverter
import org.springframework.data.mongodb.core.convert.MongoCustomConversions
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.temporal.ChronoUnit
import java.util.Date

@Configuration
class MongoConfig {
    @Bean
    fun mongoCustomConversions(): MongoCustomConversions {
        val converters: MutableList<Converter<*, *>?> = ArrayList()
        converters.add(ZonedDateTimeToDate.INSTANCE)
        converters.add(DateToZonedDateTime.INSTANCE)

        return MongoCustomConversions(converters)
    }

    @ReadingConverter
    internal enum class DateToZonedDateTime : Converter<Date?, ZonedDateTime?> {
        INSTANCE,
        ;

        override fun convert(date: Date): ZonedDateTime =
            date
                .toInstant()
                .atZone(ZoneId.systemDefault())
                .truncatedTo(ChronoUnit.MILLIS)
    }

    @WritingConverter
    internal enum class ZonedDateTimeToDate : Converter<ZonedDateTime?, Date?> {
        INSTANCE,
        ;

        override fun convert(zonedDateTime: ZonedDateTime): Date = Date.from(zonedDateTime.toInstant())
    }
}

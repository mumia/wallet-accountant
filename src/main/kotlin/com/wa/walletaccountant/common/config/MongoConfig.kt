package com.wa.walletaccountant.common.config

import com.wa.walletaccountant.common.converter.ZonedDateTimeReadConverter
import com.wa.walletaccountant.common.converter.ZonedDateTimeWriteConverter
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.core.convert.converter.Converter
import org.springframework.data.mongodb.core.convert.MongoCustomConversions

@Configuration
class MongoConfig {
    @Bean
    fun mongoCustomConversions(
        zonedDateTimeReadConverter: ZonedDateTimeReadConverter,
        zonedDateTimeWriteConverter: ZonedDateTimeWriteConverter,
    ): MongoCustomConversions {
        val converters: MutableList<Converter<*, *>?> = ArrayList()
        converters.add(zonedDateTimeReadConverter)
        converters.add(zonedDateTimeWriteConverter)

        return MongoCustomConversions(converters)
    }
}

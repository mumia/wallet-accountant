package com.wa.walletaccountant.common.config

import com.wa.walletaccountant.common.converter.BigDecimalReadConverter
import com.wa.walletaccountant.common.converter.BigDecimalWriteConverter
import com.wa.walletaccountant.common.converter.YearReadConverter
import com.wa.walletaccountant.common.converter.YearWriteConverter
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
        yearReadConverter: YearReadConverter,
        yearWriteConverter: YearWriteConverter,
        bigDecimalReadConverter: BigDecimalReadConverter,
        bigDecimalWriteConverter: BigDecimalWriteConverter,
    ): MongoCustomConversions {
        val converters: MutableList<Converter<*, *>?> = ArrayList()
        converters.add(zonedDateTimeReadConverter)
        converters.add(zonedDateTimeWriteConverter)
        converters.add(yearReadConverter)
        converters.add(yearWriteConverter)
        converters.add(bigDecimalReadConverter)
        converters.add(bigDecimalWriteConverter)

        return MongoCustomConversions(converters)
    }
}

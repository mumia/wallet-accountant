package com.walletaccountant.application.configuration

import com.walletaccountant.domain.shared.Money
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration
import org.springframework.core.convert.converter.Converter
import org.springframework.data.mongodb.core.convert.MongoCustomConversions
import java.math.BigDecimal
import java.time.Year

@Configuration
class MongoConfiguration {

    @Bean
    fun mongoCustomConversions(): MongoCustomConversions {
        return MongoCustomConversions(
            listOf(
                MoneyToBigDecimalConverter(),
                BigDecimalToMoneyConverter(),
                YearToIntConverter(),
                IntToYearConverter()
            )
        )
    }

    class MoneyToBigDecimalConverter : Converter<Money, BigDecimal> {
        override fun convert(source: Money): BigDecimal = source.amount
    }

    class BigDecimalToMoneyConverter : Converter<BigDecimal, Money> {
        override fun convert(source: BigDecimal): Money = Money.of(source)
    }

    class YearToIntConverter : Converter<Year, Int> {
        override fun convert(source: Year): Int = source.value
    }

    class IntToYearConverter : Converter<Int, Year> {
        override fun convert(source: Int): Year = Year.of(source)
    }
}

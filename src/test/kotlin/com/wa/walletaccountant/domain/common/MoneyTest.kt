package com.wa.walletaccountant.domain.common

import com.fasterxml.jackson.core.JsonProcessingException
import com.fasterxml.jackson.databind.ObjectMapper
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertNotEquals
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.skyscreamer.jsonassert.JSONAssert
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import java.util.stream.Stream

@SpringBootTest(classes = [ObjectMapper::class])
class MoneyTest {
    @Autowired
    private lateinit var objectMapper: ObjectMapper

    private val serializedMoney = "1000.16"
    private val deserializedMoney = Money(1000.16)

    @Test
    fun testSerialization() {
        val serializedJson: String = objectMapper.writeValueAsString(deserializedMoney)
        JSONAssert.assertEquals(serializedMoney, serializedJson, false)
    }

    @Test
    @Throws(JsonProcessingException::class)
    fun testDeserialization() {
        val actualInstance: Money = objectMapper.readValue(serializedMoney, Money::class.java)
        assertEquals(deserializedMoney, actualInstance)
    }

    @Test
    fun shouldBeSame() {
        val money1 = Money(10.231)
        val money2 = Money(10.23)

        assertEquals(money1, money2)
        assertEquals(money1.hashCode(), money2.hashCode())
        assertTrue(money1 == money2)
        assertEquals("10.23", money1.toString())
    }

    @ParameterizedTest
    @MethodSource("differentMoney")
    fun shouldNotBeSame(
        money1: Money,
        money2: Money,
    ) {
        assertNotEquals(money1, money2)
        assertNotEquals(money1.hashCode(), money2.hashCode())
        assertFalse { money1 == money2 }
    }

    companion object {
        @JvmStatic
        fun differentMoney(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    Money(10.00),
                    Money(10.01),
                ),
                Arguments.of(
                    Money(20.0),
                    Money(10.0),
                ),
            )
    }
}

package com.wa.walletaccountant.domain.account.account

import com.fasterxml.jackson.core.JsonProcessingException
import com.fasterxml.jackson.databind.ObjectMapper
import org.junit.jupiter.api.Assertions
import org.skyscreamer.jsonassert.JSONAssert
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import kotlin.test.Test

@SpringBootTest(classes = [ObjectMapper::class])
class AccountTypeTest {
    @Autowired
    private val objectMapper: ObjectMapper? = null

    private val json =
        """
        "CHECKING"
        """.trimIndent()

    @Test
    fun testSerialization() {
        val serializedJson: String = objectMapper!!.writeValueAsString(AccountType.CHECKING)
        JSONAssert.assertEquals(json, serializedJson, false)
    }

    @Test
    @Throws(JsonProcessingException::class)
    fun testDeserialization() {
        val actualInstance: AccountType = objectMapper!!.readValue(json, AccountType::class.java)

        Assertions.assertEquals(AccountType.CHECKING, actualInstance)
    }
}

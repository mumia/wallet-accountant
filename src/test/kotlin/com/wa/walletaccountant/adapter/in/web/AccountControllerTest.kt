package com.wa.walletaccountant.adapter.`in`.web

import com.ninjasquad.springmockk.MockkBean
import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.query.account.ReadAccountById
import com.wa.walletaccountant.application.query.account.ReadAllAccounts
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.account.account.AccountType.CHECKING
import com.wa.walletaccountant.domain.account.account.AccountType.SAVINGS
import com.wa.walletaccountant.domain.account.account.BankName.BCP
import com.wa.walletaccountant.domain.account.account.BankName.N26
import com.wa.walletaccountant.domain.account.command.RegisterNewAccountCommand
import com.wa.walletaccountant.domain.common.Currency.EUR
import com.wa.walletaccountant.domain.common.Currency.USD
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import io.mockk.confirmVerified
import io.mockk.every
import io.mockk.verify
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.messaging.responsetypes.ResponseType
import org.axonframework.messaging.responsetypes.ResponseTypes
import org.axonframework.queryhandling.QueryGateway
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.http.MediaType
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.MvcResult
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders
import org.springframework.test.web.servlet.result.MockMvcResultHandlers
import org.springframework.test.web.servlet.result.MockMvcResultMatchers
import java.util.Optional
import java.util.UUID
import java.util.concurrent.CompletableFuture
import java.util.stream.Stream

@AutoConfigureMockMvc
@SpringBootTest
class AccountControllerTest {
    @Autowired
    lateinit var mockMvc: MockMvc

    @MockkBean
    lateinit var commandGateway: CommandGateway

    @MockkBean
    lateinit var queryGateway: QueryGateway

    @MockkBean
    lateinit var idGenerator: IdGenerator

    val expectedFailureResponse =
        """
        {
        	"type": "ConstraintViolation",
        	"title": "A validation constraint failed",
        	"constraint-violations": [
        		{
        			"name": "startingBalanceDate",
        			"value": "201-08-26",
        			"reason": "Expected date format is YYYY-MM-DD"
        		},
        		{
        			"name": "startingBalance",
        			"value": "-1",
        			"reason": "must be greater than or equal to 0"
        		},
        		{
        			"name": "name",
        			"value": "",
        			"reason": "must not be empty"
        		},
        		{
        			"name": "accountType",
        			"value": "checking",
        			"reason": "Invalid account type found, expected one of CHECKING, SAVINGS"
        		},
        		{
        			"name": "currency",
        			"value": "eur",
        			"reason": "Invalid currency found, expected one of EUR, USD, CHF"
        		},
        		{
        			"name": "bankName",
        			"value": "bcp",
        			"reason": "Invalid bank name found, expected one of DB, N26, BCP"
        		}
        	]
        }
        """.trimIndent()

    @Test
    fun testRegistrationValidations() {
        val request =
            """
            {
                "bankName": "bcp",
                "name": "",
                "accountType": "checking",
                "startingBalance": -1,
                "startingBalanceDate": "201-08-26",
                "currency": "eur"
            }
            """.trimIndent()

        mockMvc
            .perform(
                MockMvcRequestBuilders
                    .post("/api/v1/account")
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(request),
            ).andExpect(MockMvcResultMatchers.status().isBadRequest)
            .andExpect(MockMvcResultMatchers.content().json(expectedFailureResponse))
    }

    @Test
    fun testSuccessfulRegistration() {
        val accountId = AccountId(UUID.randomUUID())

        val request =
            """
            {
                "bankName": "BCP",
                "name": "Mumia bcp",
                "accountType": "CHECKING",
                "startingBalance": 1000.5,
                "startingBalanceDate": "2014-08-26",
                "currency": "EUR",
                "notes": "some notes"
            }
            """.trimIndent()

        val response =
            """
            {
                "accountId": "%s"
            }
            """.trimIndent().format(accountId.value.toString())

        val command =
            RegisterNewAccountCommand(
                accountId = accountId,
                bankName = BCP,
                name = "Mumia bcp",
                accountType = CHECKING,
                startingBalance = Money(1000.5),
                startingBalanceDate = startingBalanceDate1,
                currency = EUR,
                notes = "some notes",
            )

        every { commandGateway.send<AccountId>(any()) } returns CompletableFuture.completedFuture(accountId)
        every { idGenerator.newId() } returns accountId.value

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders
                        .post("/api/v1/account")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(request),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) { commandGateway.send<AccountId>(command) }

        confirmVerified(commandGateway)
    }

    @Test
    fun testReadAccountValidations() {
        mockMvc
            .perform(MockMvcRequestBuilders.get("/api/v1/account/%s".format("asdf")))
            .andExpect(MockMvcResultMatchers.status().isBadRequest)
            .andExpect(
                MockMvcResultMatchers
                    .content()
                    .json(
                        """
                        {
                            "type": "ConstraintViolation",
                            "title": "A validation constraint failed",
                            "constraint-violations": [
                                {
                                    "name": "readAccount.accountId",
                                    "value": "asdf",
                                    "reason": "Invalid account id, must be a valid UUID"
                                }
                            ]
                        }
                        """.trimIndent(),
                    ),
            )

        verify(atLeast = 0, atMost = 0) {
            queryGateway.query(
                ofType(ReadAccountById::class),
                ResponseTypes.optionalInstanceOf(AccountModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    @Disabled("for some reason Mockmvc always sees the response code 200, but the controller is returning 404...,")
    @Test
    fun testReadAccountNotFound() {
        every {
            queryGateway.query(ofType(ReadAccountById::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(Optional.empty<AccountModel>())

        mockMvc
            .perform(MockMvcRequestBuilders.get("/api/v1/account/6047c24c-b6ae-4f38-ae7d-791a845bc124"))
            .andExpect(MockMvcResultMatchers.status().isNotFound)
//            .andExpect(
//                MockMvcResultMatchers
//                    .content()
//                    .json(
//                        """
//                        {
//                            "type": "ConstraintViolation",
//                            "title": "A validation constraint failed",
//                            "constraint-violations": [
//                                {
//                                    "name": "readAccount.accountId",
//                                    "value": "asdf",
//                                    "reason": "Invalid account id, must be a valid UUID"
//                                }
//                            ]
//                        }
//                        """.trimIndent(),
//                    ),
//            )

        verify(atLeast = 0, atMost = 0) {
            queryGateway.query(
                ofType(ReadAccountById::class),
                ResponseTypes.optionalInstanceOf(AccountModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    @Test
    fun testSuccessfulReadAccount() {
        val accountId = AccountId(UUID.randomUUID())

        val response =
            """
            {
                "accountId": "%s",
                "bankName": "BCP",
                "name": "Mumia bcp",
                "accountType": "CHECKING",
                "startingBalance": 1000.50,
                "startingBalanceDate": "2014-08-26",
                "currency": "EUR",
                "notes": "some notes",
                "currentMonth": "2014-08-01"
            }
            """.trimIndent()
                .format(accountId.value.toString())

        val model =
            AccountModel(
                accountId = accountId,
                bankName = BCP,
                name = "Mumia bcp",
                accountType = CHECKING,
                startingBalance = Money(amount = 1000.50),
                startingBalanceDate = startingBalanceDate1,
                currency = EUR,
                notes = "some notes",
                currentMonth = Date.fromMonthYEar(startingBalanceDate1.month(), startingBalanceDate1.year()),
            )

        val query = ReadAccountById(accountId = accountId)

        every {
            queryGateway.query(ofType(ReadAccountById::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(Optional.of(model))

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders.get("/api/v1/account/%s".format(accountId.value.toString())),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) {
            queryGateway.query(query, ResponseTypes.optionalInstanceOf(AccountModel::class.java))
        }

        confirmVerified(queryGateway)
    }

    @ParameterizedTest
    @MethodSource("multipleAccounts")
    fun testSuccessfulReadAllAccount(
        models: List<AccountModel>,
        response: String,
    ) {
        every {
            queryGateway.query(ofType(ReadAllAccounts::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(models)

        val result: MvcResult =
            mockMvc
                .perform(MockMvcRequestBuilders.get("/api/v1/accounts"))
                .andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) {
            queryGateway.query(
                ofType(ReadAllAccounts::class),
                ResponseTypes.multipleInstancesOf(AccountModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    companion object {
        val startingBalanceDate1 = Date.fromString("2014-08-26")
        val startingBalanceDate2 = Date.fromString("2013-12-26")

        @JvmStatic
        fun multipleAccounts(): Stream<Arguments> {
            val accountId1 = AccountId(UUID.randomUUID())
            val accountId2 = AccountId(UUID.randomUUID())

            val account1Response =
                """
                {
                    "accountId": "%s",
                    "bankName": "BCP",
                    "name": "Mumia bcp",
                    "accountType": "CHECKING",
                    "startingBalance": 1000.50,
                    "startingBalanceDate": "2014-08-26",
                    "currency": "EUR",
                    "notes": "some notes",
                    "currentMonth": "2014-08-01"
                }
                """.trimIndent()
                    .format(accountId1.value.toString())

            val account2Response =
                """
                {
                    "accountId": "%s",
                    "bankName": "N26",
                    "name": "Mumia n26",
                    "accountType": "SAVINGS",
                    "startingBalance": 101.09,
                    "startingBalanceDate": "2013-12-26",
                    "currency": "USD",
                    "currentMonth": "2013-12-01"
                }
                """.trimIndent()
                    .format(accountId2.value.toString())

            val response = "[%s,%s]"

            val model1 =
                AccountModel(
                    accountId = accountId1,
                    bankName = BCP,
                    name = "Mumia bcp",
                    accountType = CHECKING,
                    startingBalance = Money(amount = 1000.50),
                    startingBalanceDate = startingBalanceDate1,
                    currency = EUR,
                    notes = "some notes",
                    currentMonth = Date.fromMonthYEar(startingBalanceDate1.month(), startingBalanceDate1.year()),
                )
            val model2 =
                AccountModel(
                    accountId = accountId2,
                    bankName = N26,
                    name = "Mumia n26",
                    accountType = SAVINGS,
                    startingBalance = Money(amount = 101.09),
                    startingBalanceDate = startingBalanceDate2,
                    currency = USD,
                    notes = "",
                    currentMonth = Date.fromMonthYEar(startingBalanceDate2.month(), startingBalanceDate2.year()),
                )

            return Stream.of(
                Arguments.of(
                    emptyList<AccountModel>(),
                    "[]",
                ),
                Arguments.of(
                    listOf(model1),
                    "[%s]".format(account1Response),
                ),
                Arguments.of(
                    listOf(model2, model1),
                    "[%s, %s]".format(account2Response, account1Response),
                ),
            )
        }
    }
}

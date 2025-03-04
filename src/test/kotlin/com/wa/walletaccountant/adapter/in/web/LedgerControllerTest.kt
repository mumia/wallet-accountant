package com.wa.walletaccountant.adapter.`in`.web

import com.ninjasquad.springmockk.MockkBean
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.model.ledger.LedgerTransactionModel
import com.wa.walletaccountant.application.query.ledger.ReadCurrentMonthLedger
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Date
import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.ledger.command.RegisterTransactionCommand
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import io.mockk.confirmVerified
import io.mockk.every
import io.mockk.verify
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.messaging.responsetypes.ResponseType
import org.axonframework.messaging.responsetypes.ResponseTypes
import org.axonframework.queryhandling.QueryGateway
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.http.MediaType
import org.springframework.test.web.servlet.MockMvc
import org.springframework.test.web.servlet.MvcResult
import org.springframework.test.web.servlet.request.MockMvcRequestBuilders
import org.springframework.test.web.servlet.result.MockMvcResultHandlers
import org.springframework.test.web.servlet.result.MockMvcResultMatchers
import java.time.Month
import java.time.Month.MAY
import java.time.Year
import java.util.Optional
import java.util.UUID
import java.util.concurrent.CompletableFuture

@AutoConfigureMockMvc
@SpringBootTest
class LedgerControllerTest {
    @Autowired
    lateinit var mockMvc: MockMvc

    @MockkBean
    lateinit var commandGateway: CommandGateway

    @MockkBean
    lateinit var queryGateway: QueryGateway


    @MockkBean
    lateinit var idGenerator: IdGenerator

    @Test
    fun testTransactionRegistrationValidations() {
        val expectedFailureResponse =
            """
        {
          "type": "ConstraintViolation",
          "title": "A validation constraint failed",
          "constraint-violations": [
            {
              "name": "description",
              "value": "",
              "reason": "must not be empty"
            },
            {
              "name": "date",
              "value": "201-08-26",
              "reason": "Expected date format is YYYY-MM-DD"
            },
            {
              "name": "accountId",
              "value": "not an uuid",
              "reason": "must be a valid UUID"
            },
            {
              "name": "sourceAccountId",
              "value": "not an uuid",
              "reason": "must be a valid UUID"
            },
            {
              "name": "movementTypeId",
              "value": "not an uuid",
              "reason": "must be a valid UUID"
            },
            {
              "name": "tagIds[].<iterable element>",
              "value": "not an uuid",
              "reason": "must be a valid UUID"
            }
          ]
        }
        """.trimIndent()

        val request =
            """
            {
                "accountId": "not an uuid",
                "movementTypeId": "not an uuid",
                "amount": 1,
                "date": "201-08-26",
                "sourceAccountId": "not an uuid",
                "description": "",
                "notes": 10,
                "tagIds": ["not an uuid"]
            }
            """.trimIndent()

        mockMvc
            .perform(
                MockMvcRequestBuilders
                    .post("/api/v1/ledger/transaction")
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(request),
            ).andExpect(MockMvcResultMatchers.status().isBadRequest)
            .andExpect(MockMvcResultMatchers.content().json(expectedFailureResponse))
    }

    @Test
    fun testSuccessfulTransactionRegistration() {
        val accountId = AccountId(UUID.randomUUID())
        val tagId = TagId(UUID.randomUUID())
        val transactionId = TransactionId(UUID.randomUUID())
        val date = Date.fromString("2025-02-26")

        val request =
            """
            {
                "accountId": "%s",
                "amount": 10.25,
                "date": "%s",
                "description": "a description",
                "tagIds": ["%s"]
            }
            """.trimIndent().format(accountId.toString(), date.toString(), tagId.toString())

        val response =
            """
            {
                "transactionId": "%s"
            }
            """.trimIndent().format(transactionId.value.toString())

        val command =
            RegisterTransactionCommand(
                ledgerId = LedgerId(
                    accountId = accountId,
                    month = Month.FEBRUARY,
                    year = Year.of(2025),
                ),
                transactionId =  transactionId,
                movementTypeId = null,
                amount = Money(10.25),
                date = date,
                sourceAccountId = null,
                description = "a description",
                notes = null,
                tagIds = setOf(tagId),
            )

        every { commandGateway.send<TransactionId>(any()) }returns CompletableFuture.completedFuture(
            transactionId
        )
        every { idGenerator.newId() } returns transactionId.value

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders
                        .post("/api/v1/ledger/transaction")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(request),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) { commandGateway.send<TransactionId>(command) }

        confirmVerified(commandGateway)
    }

    @Test
    fun testReadCurrentMonthLedgerValidations() {
        mockMvc
            .perform(MockMvcRequestBuilders.get("/api/v1/ledger/%s".format("asdf")))
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
                                    "name": "readCurrentMonthLedger.accountId",
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
                ofType(ReadCurrentMonthLedger::class),
                ResponseTypes.optionalInstanceOf(LedgerMonthModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    @Test
    fun testSuccessfulReadCurrentMonthLedger() {
        val accountId = AccountId.fromString("83bf7fcb-d537-4596-921d-5e7103d2bd3f")
        val ledgerId = LedgerId(
            accountId = accountId,
            month = MAY,
            year = Year.of(2025)
        )
        val transactionId = TransactionId.fromString("44df052d-13fc-4900-b792-6d5075fe1973")
        val dateStr = "2025-02-22"
        val description = "transaction description"
        val tagId = TagId.fromString("17c9575b-b43e-4c7f-bbf5-a7f36bc1af1f")

        val response =
            """
            {
                "ledgerId": {"accountId":"83bf7fcb-d537-4596-921d-5e7103d2bd3f","month":"MAY","year":"2025"},
                "balance": 1000.5,
                "transactions": [
                    {
                        "transactionId": "%s",
                        "movementTypeId": null,
                        "action": "Debit",
                        "amount": 100.12,
                        "date": "%s",
                        "sourceAccountId": null,
                        "description": "%s",
                        "notes": null,
                        "tagIds": ["%s"]
                    }
                ],
                "closed": true
            }
            """.trimIndent()
                .format(
                    transactionId.toString(),
                    dateStr,
                    description,
                    tagId.toString()
                )

        val model =
            LedgerMonthModel(
                ledgerId = ledgerId,
                balance = Money(amount = 1000.50),
                transactions = setOf(
                    LedgerTransactionModel(
                        transactionId = transactionId,
                        movementTypeId = null,
                        action = Debit,
                        amount = Money(amount = 100.12),
                        date = Date.fromString(dateStr),
                        sourceAccountId = null,
                        description = description,
                        notes = null,
                        tagIds = setOf(tagId)
                    )
                ),
                closed = true,
            )

        val query = ReadCurrentMonthLedger(accountId = accountId)

        every {
            queryGateway.query(ofType(ReadCurrentMonthLedger::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(Optional.of(model))

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders.get("/api/v1/ledger/%s".format(accountId.value.toString())),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) {
            queryGateway.query(query, ResponseTypes.optionalInstanceOf(LedgerMonthModel::class.java))
        }

        confirmVerified(queryGateway)
    }
}

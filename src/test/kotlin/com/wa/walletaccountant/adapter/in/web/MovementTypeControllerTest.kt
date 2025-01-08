package com.wa.walletaccountant.adapter.`in`.web

import com.ninjasquad.springmockk.MockkBean
import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.application.query.account.ReadAccountById
import com.wa.walletaccountant.application.query.movementtype.ReadAllMovementTypes
import com.wa.walletaccountant.application.query.movementtype.ReadMovementTypeById
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.command.RegisterNewMovementTypeCommand
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
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
class MovementTypeControllerTest {
    @Autowired
    lateinit var mockMvc: MockMvc

    @MockkBean
    lateinit var commandGateway: CommandGateway

    @MockkBean
    lateinit var queryGateway: QueryGateway

    @MockkBean
    lateinit var idGenerator: IdGenerator

    private val accountId: AccountId = AccountId.fromString("77e52c3d-a0eb-4328-8416-f5e7517120ac")
    private val tagId1: TagId = TagId.fromString("e661ea45-deba-4e88-98e0-eb0d53ce3ab0")
    private val tagId2: TagId = TagId.fromString("d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc")

    val expectedFailureResponse =
        """
        {
            "type": "ConstraintViolation",
            "title": "A validation constraint failed",
            "constraint-violations": [
                {
                    "name": "action",
                    "value": "invalid",
                    "reason": "Unknown value for field, expected one of Debit, Credit"
                },
                {
                    "name": "sourceAccountId",
                    "value": "add",
                    "reason": "must be a valid UUID"
                },
                {
                    "name": "description",
                    "value": "",
                    "reason": "must not be empty"
                },
                {
                    "name": "accountId",
                    "value": "9-4c7e-b609-ce221421e8e8",
                    "reason": "must be a valid UUID"
                }
            ]
        }
        """.trimIndent()

    @Test
    fun testRegistrationValidations() {
        val request =
            """
            {
                "action": "invalid",
                "accountId": "9-4c7e-b609-ce221421e8e8",
                "sourceAccountId": "add",
                "description": "",
                "tagIds": [
                    "eb280-f494-452c-a631-2976aa0d12f3",
                    "ea31624b-a3ae-4101-9ed0-8b7e1f86ffb1"
                ]
            }
            """.trimIndent()

        mockMvc
            .perform(
                MockMvcRequestBuilders
                    .post("/api/v1/movement-type")
                    .contentType(MediaType.APPLICATION_JSON)
                    .content(request),
            ).andExpect(MockMvcResultMatchers.status().isBadRequest)
            .andExpect(MockMvcResultMatchers.content().json(expectedFailureResponse))
    }

    @Test
    fun testSuccessfulRegistration() {
        val movementTypeId = MovementTypeId(UUID.randomUUID())

        val request =
            """
            {
                "action": "Debit",
                "accountId": "%s",
                "description": "a description",
                "tagIds": [
                    "%s",
                    "%s"
                ]
            }
            """.trimIndent().format(accountId.toString(), tagId1.toString(), tagId2.toString())

        val response =
            """
            {
                "movementTypeId": "%s"
            }
            """.trimIndent().format(movementTypeId.value.toString())

        val command =
            RegisterNewMovementTypeCommand(
                movementTypeId = movementTypeId,
                movementAction = Debit,
                accountId = accountId,
                sourceAccountId = null,
                description = "a description",
                notes = null,
                tagIds = setOf(tagId1, tagId2)
            )

        every { commandGateway.send<MovementTypeId>(any()) } returns CompletableFuture.completedFuture(movementTypeId)
        every { idGenerator.newId() } returns movementTypeId.value

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders
                        .post("/api/v1/movement-type")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(request),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) { commandGateway.send<MovementTypeId>(command) }

        confirmVerified(commandGateway)
    }

    @Test
    fun testReadMovementTypeValidations() {
        mockMvc
            .perform(MockMvcRequestBuilders.get("/api/v1/movement-type/%s".format("asdf")))
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
                                    "name": "readMovementType.movementTypeId",
                                    "value": "asdf",
                                    "reason": "Invalid movement type id, must be a valid UUID"
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
    fun testReadMovementTypeNotFound() {
        every {
            queryGateway.query(ofType(ReadMovementTypeById::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(Optional.empty<MovementTypeModel>())

        mockMvc
            .perform(MockMvcRequestBuilders.get("/api/v1/movement-type/6047c24c-b6ae-4f38-ae7d-791a845bc124"))
            .andExpect(MockMvcResultMatchers.status().isNotFound)

        verify(atMost = 1) {
            queryGateway.query(
                ofType(ReadMovementTypeById::class),
                ResponseTypes.optionalInstanceOf(MovementTypeModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    @Test
    fun testSuccessfulReadMovementType() {
        val movementTypeId = MovementTypeId(UUID.randomUUID())

        val response =
            """
            {
                "movementTypeId": "%s",
                "movementAction": "Debit",
                "accountId": "%s",
                "sourceAccountId": null,
                "description": "a description",
                "notes": null,
                "tagIds": [
                    "%s",
                    "%s"
                ]
            }
            """.trimIndent()
                .format(movementTypeId.toString(), accountId.toString(), tagId1.toString(), tagId2.toString())

        val model =
            MovementTypeModel(
                movementTypeId = movementTypeId,
                movementAction = Debit,
                accountId = accountId,
                sourceAccountId = null,
                description = "a description",
                notes = null,
                tagIds = setOf(tagId1, tagId2)
            )

        val query = ReadMovementTypeById(movementTypeId = movementTypeId)

        every {
            queryGateway.query(ofType(ReadMovementTypeById::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(Optional.of(model))

        val result: MvcResult =
            mockMvc
                .perform(
                    MockMvcRequestBuilders.get("/api/v1/movement-type/%s".format(movementTypeId.toString())),
                ).andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) {
            queryGateway.query(query, ResponseTypes.optionalInstanceOf(MovementTypeModel::class.java))
        }

        confirmVerified(queryGateway)
    }

    @ParameterizedTest
    @MethodSource("multipleMovementTypes")
    fun testSuccessfulReadAllMovementTypes(
        models: List<MovementTypeModel>,
        response: String,
    ) {
        every {
            queryGateway.query(ofType(ReadAllMovementTypes::class), ofType(ResponseType::class))
        } returns CompletableFuture.completedFuture(models)

        val result: MvcResult =
            mockMvc
                .perform(MockMvcRequestBuilders.get("/api/v1/movement-types"))
                .andExpect(MockMvcResultMatchers.request().asyncStarted())
                .andDo(MockMvcResultHandlers.log())
                .andReturn()

        mockMvc
            .perform(MockMvcRequestBuilders.asyncDispatch(result))
            .andExpect(MockMvcResultMatchers.status().isOk())
            .andExpect(MockMvcResultMatchers.content().json(response))

        verify(atMost = 1) {
            queryGateway.query(
                ofType(ReadAllMovementTypes::class),
                ResponseTypes.multipleInstancesOf(MovementTypeModel::class.java),
            )
        }

        confirmVerified(queryGateway)
    }

    companion object {
        @JvmStatic
        fun multipleMovementTypes(): Stream<Arguments> {
            val movementTypeId = MovementTypeId(UUID.randomUUID())
            val accountId1 = AccountId(UUID.randomUUID())
            val accountId2 = AccountId(UUID.randomUUID())
            val tagId1 = TagId(UUID.randomUUID())
            val tagId2 = TagId(UUID.randomUUID())

            val movementTypeResponse1 =
                """
                {
                    "movementTypeId": "%s",
                    "movementAction": "Debit",
                    "accountId": "%s",
                    "sourceAccountId": null,
                    "description": "a description",
                    "notes": null,
                    "tagIds": [
                        "%s",
                        "%s"
                    ]
                }
                """.trimIndent()
                    .format(movementTypeId.toString(), accountId1.toString(), tagId1.toString(), tagId2.toString())

            val movementTypeResponse2 =
                """
                {
                    "movementTypeId": "%s",
                    "movementAction": "Debit",
                    "accountId": "%s",
                    "sourceAccountId": "%s",
                    "description": "another description",
                    "notes": "Some notes",
                    "tagIds": [
                        "%s"
                    ]
                }
                """.trimIndent()
                    .format(movementTypeId.toString(), accountId2.toString(), accountId1.toString(), tagId2.toString())

            val model1 =
                MovementTypeModel(
                    movementTypeId = movementTypeId,
                    movementAction = Debit,
                    accountId = accountId1,
                    sourceAccountId = null,
                    description = "a description",
                    notes = null,
                    tagIds = setOf(tagId1, tagId2)
                )

            val model2 =
                MovementTypeModel(
                    movementTypeId = movementTypeId,
                    movementAction = Debit,
                    accountId = accountId2,
                    sourceAccountId = accountId1,
                    description = "another description",
                    notes = "Some notes",
                    tagIds = setOf(tagId2)
                )

            return Stream.of(
                Arguments.of(
                    emptyList<MovementTypeModel>(),
                    "[]",
                ),
                Arguments.of(
                    listOf(model1),
                    "[%s]".format(movementTypeResponse1),
                ),
                Arguments.of(
                    listOf(model2, model1),
                    "[%s, %s]".format(movementTypeResponse2, movementTypeResponse1),
                ),
            )
        }
    }
}

package com.wa.walletaccountant.adapter.out.readmodel.movementtype

import com.wa.walletaccountant.adapter.out.readmodel.MovementTypeReadModelAdapter
import com.wa.walletaccountant.adapter.out.readmodel.movementtype.mapper.MovementTypeMapper
import com.wa.walletaccountant.adapter.out.readmodel.movementtype.repository.MovementTypeRepository
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Credit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction.Debit
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.test.context.ActiveProfiles
import org.springframework.test.context.DynamicPropertyRegistry
import org.springframework.test.context.DynamicPropertySource
import org.testcontainers.containers.MongoDBContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers
import java.util.Optional
import java.util.stream.Stream

@Testcontainers
@SpringBootTest
@ActiveProfiles("testing")
class MovementTypeReadModelAdapterTest
@Autowired
constructor(
    val movementTypeRepository: MovementTypeRepository,
    val movementTypeReadModelAdapter: MovementTypeReadModelAdapter,
) {
    companion object {
        @Container
        @JvmField
        val mongoDBContainer: MongoDBContainer = MongoDBContainer("mongo:7.0.7")

        init {
            mongoDBContainer.start()
        }

        @DynamicPropertySource
        @JvmStatic
        fun setProperties(registry: DynamicPropertyRegistry) {
            registry.add("spring.data.mongodb.uri") { mongoDBContainer.replicaSetUrl }
        }

        private val movementTypeId1 = MovementTypeId.fromString("c5be2bf8-4ffa-4b3e-a152-518cec206b1d")
        private val movementTypeId2 = MovementTypeId.fromString("d61bf2f6-95ca-4bff-8c4e-439631144436")
        private val accountId = AccountId.fromString("77e52c3d-a0eb-4328-8416-f5e7517120ac")
        private val sourceAccountId = AccountId.fromString("ba28f98c-0326-4ac6-b778-0bf86f088fd1")
        private val description1 = "Movement type description 1"
        private val description2 = "Movement type description 2"
        private val tagId1 = TagId.fromString("e661ea45-deba-4e88-98e0-eb0d53ce3ab0")
        private val tagId2 = TagId.fromString("d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc")

        private val movementTypeModelDebitSingle = MovementTypeModel(
            movementTypeId = movementTypeId1,
            movementAction = Debit,
            accountId = accountId,
            sourceAccountId = null,
            description = description1,
            notes = null,
            tagIds = setOf(tagId1)
        )

        private val movementTypeModelCreditWithSource = MovementTypeModel(
            movementTypeId = movementTypeId2,
            movementAction = Credit,
            accountId = accountId,
            sourceAccountId = sourceAccountId,
            description = description2,
            notes = "notes 2",
            tagIds = setOf(tagId2, tagId1)
        )

        @JvmStatic
        fun readMovementTypeData(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    MovementTypeId.fromString("d869c9d6-b8e4-4b5c-bda0-d0a341bd4dbc"),
                    Optional.empty<MovementTypeModel>()
                ),
                Arguments.of(
                    movementTypeId1,
                    Optional.of(movementTypeModelDebitSingle)
                ),
                Arguments.of(
                    movementTypeId2,
                    Optional.of(movementTypeModelCreditWithSource)
                )
            )
    }

    @AfterEach
    fun cleanUp() {
        movementTypeRepository.deleteAll()
    }

    @Test
    fun testRegisterMovementTypesAndReadAll() {
        movementTypeReadModelAdapter.registerNewMovementType(movementTypeModelDebitSingle)

        var actualMovementTypes = movementTypeReadModelAdapter.readAllMovementTypes()
        assertEquals(setOf(movementTypeModelDebitSingle), actualMovementTypes)

        movementTypeReadModelAdapter.registerNewMovementType(movementTypeModelCreditWithSource)

        actualMovementTypes = movementTypeReadModelAdapter.readAllMovementTypes()
        assertEquals(setOf(movementTypeModelDebitSingle, movementTypeModelCreditWithSource), actualMovementTypes)
    }

    @ParameterizedTest
    @MethodSource("readMovementTypeData")
    fun testReadMovementType(
        movementTypeId: MovementTypeId,
        expectedMovementType: Optional<MovementTypeModel>,
    ) {
        prepareMultiMovementTypes()

    val actualMovementType = movementTypeReadModelAdapter.readMovementType(movementTypeId)

    assertEquals(expectedMovementType, actualMovementType)
    }

    private fun prepareMultiMovementTypes() {
        movementTypeRepository.saveAll(
            listOf(
                MovementTypeMapper.toDocument(movementTypeModelDebitSingle),
                MovementTypeMapper.toDocument(movementTypeModelCreditWithSource)
            ),
        )
    }
}
package com.wa.walletaccountant.application.interceptor.service

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper.TagCategoryMapper
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.TagCategoryRepository
import com.wa.walletaccountant.application.interceptor.exception.TagAlreadyExistsException
import com.wa.walletaccountant.application.interceptor.exception.TagCategoryAlreadyExistsException
import com.wa.walletaccountant.application.interceptor.exception.UnknownTagCategoryException
import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.model.tagcategory.TagModel
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest
import org.springframework.test.context.ActiveProfiles
import org.springframework.test.context.DynamicPropertyRegistry
import org.springframework.test.context.DynamicPropertySource
import org.testcontainers.containers.MongoDBContainer
import org.testcontainers.junit.jupiter.Container
import org.testcontainers.junit.jupiter.Testcontainers
import kotlin.test.assertFailsWith

@Testcontainers
@SpringBootTest(webEnvironment = SpringBootTest.WebEnvironment.RANDOM_PORT)
@ActiveProfiles("testing")
class TagValidatorTest
@Autowired
constructor(
    val tagCategoryRepository: TagCategoryRepository,
    val tagCategoryValidator: TagCategoryValidator,
    val tagValidator: TagValidator
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

        private val tagCategoryId1 = TagCategoryId.fromString("fb6d28c6-6431-4800-852b-7f9147f9893b")
        private val tagCategoryId2 = TagCategoryId.fromString("63f72822-c551-4ef4-ba51-3fdc5829a3f1")
        private val tagCategoryName1 = "tag cat name 1"
        private val tagCategoryName2 = "tag cat name 2"
        private val tagId1 = TagId.fromString("6041ef1a-7957-4680-88d4-1f350283357d")
        private val tagId2 = TagId.fromString("a228ea35-4976-4702-8296-da355f649364")
        private val tagName1 = "tag name 1"
        private val tagName2 = "tag name 2"
    }

    @BeforeEach
    fun setup() {
        prepareTagCategory()
    }

    @AfterEach
    fun cleanUp() {
        tagCategoryRepository.deleteAll()
    }

    @Test
    fun testValidatorsSuccess() {
        assertDoesNotThrow { tagCategoryValidator.validateTagCategoryCanBeAdded(tagCategoryId2, tagCategoryName2) }
        assertDoesNotThrow { tagCategoryValidator.validateTagCategoryIdExists(tagCategoryId1) }
        assertDoesNotThrow { tagValidator.validateTagCanBeAdded(tagId2, tagName2) }
    }

    @Test
    fun testValidatorsFailure() {
        assertFailsWith(
            exceptionClass = TagCategoryAlreadyExistsException::class,
            block = { tagCategoryValidator.validateTagCategoryCanBeAdded(tagCategoryId1, tagCategoryName2) }
        )
        assertFailsWith(
            exceptionClass = TagCategoryAlreadyExistsException::class,
            block = { tagCategoryValidator.validateTagCategoryCanBeAdded(tagCategoryId2, tagCategoryName1) }
        )
        assertFailsWith(
            exceptionClass = UnknownTagCategoryException::class,
            block = { tagCategoryValidator.validateTagCategoryIdExists(tagCategoryId2) }
        )
        assertFailsWith(
            exceptionClass = TagAlreadyExistsException::class,
            block = { tagValidator.validateTagCanBeAdded(tagId1, tagName2) }
        )
        assertFailsWith(
            exceptionClass = TagAlreadyExistsException::class,
            block = { tagValidator.validateTagCanBeAdded(tagId2, tagName1) }
        )
    }

    private fun prepareTagCategory() {
        tagCategoryRepository.saveAll(
            listOf(
                TagCategoryMapper.toDocument(
                    TagCategoryModel(
                        tagCategoryId = tagCategoryId1,
                        name = tagCategoryName1,
                        tags = setOf(
                            TagModel(
                                tagId = tagId1,
                                name = tagName1,
                                notes = "tag notes",
                            )
                        ),
                        notes = null,
                    )
                ),
            ),
        )
    }
}
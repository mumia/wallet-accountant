package com.wa.walletaccountant.adapter.out.readmodel.tagcategory

import com.wa.walletaccountant.adapter.out.readmodel.TagCategoryReadModelAdapter
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper.TagCategoryMapper
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.TagCategoryRepository
import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.model.tagcategory.TagModel
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
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
import java.util.stream.Stream

@Testcontainers
@SpringBootTest()
@ActiveProfiles("testing")
class TagCategoryReadModelAdapterTest
@Autowired
constructor(
    val tagCategoryRepository: TagCategoryRepository,
    val tagCategoryReadModelAdapter: TagCategoryReadModelAdapter,
    val idGenerator: IdGenerator,
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
        private val tagId3 = TagId.fromString("c7d98af6-bb58-42bf-a822-5a1f2991f293")
        private val tagName1 = "tag name 1"
        private val tagName2 = "tag name 2"
        private val tagName3 = "tag name 3"
        private val tagModel1 =
            TagModel(
                tagId = tagId1,
                name = tagName1,
                notes = "tag notes",
            )
        private var tagModel2 =
            TagModel(
                tagId = tagId2,
                name = tagName2,
                notes = null,
            )
        private var tagModel3 =
            TagModel(
                tagId = tagId3,
                name = tagName3,
                notes = "",
            )

        private val tagCategory1SingleTagModel =
            TagCategoryModel(
                tagCategoryId = tagCategoryId1,
                name = tagCategoryName1,
                tags = setOf(tagModel1),
                notes = null,
            )

        private val tagCategory1MultipleTagModel =
            TagCategoryModel(
                tagCategoryId = tagCategoryId1,
                name = tagCategoryName1,
                tags = setOf(tagModel1, tagModel2),
                notes = null,
            )

        private val tagCategory2Model =
            TagCategoryModel(
                tagCategoryId = tagCategoryId2,
                name = tagCategoryName2,
                tags = setOf(tagModel3),
                notes = null,
            )

        @JvmStatic
        fun readTagsData(): Stream<Arguments> =
            Stream.of(
                Arguments.of(
                    emptySet<TagId>(),
                    setOf(
                        tagCategory1MultipleTagModel,
                        tagCategory2Model,
                    ),
                ),
                Arguments.of(
                    setOf(tagId1),
                    setOf(
                        tagCategory1SingleTagModel,
                    ),
                ),
                Arguments.of(
                    setOf(tagId1, tagId3),
                    setOf(
                        tagCategory1SingleTagModel,
                        tagCategory2Model,
                    ),
                ),
            )
    }

    @AfterEach
    fun cleanUp() {
        tagCategoryRepository.deleteAll()
    }

    @Test
    fun testTagsCanBeAddedOnNewAndExistingTagCategory() {
        tagCategoryReadModelAdapter.addNewTagToNewCategory(tagCategory1SingleTagModel)

        val actualInitialTagCategory = tagCategoryReadModelAdapter.readAllTags()
        assertEquals(setOf(tagCategory1SingleTagModel), actualInitialTagCategory)

        tagCategoryReadModelAdapter.addNewTagToExistingCategory(tagCategoryId1, tagModel2)

        val actualUpdatedTagCategory = tagCategoryReadModelAdapter.readAllTags()
        assertEquals(setOf(tagCategory1MultipleTagModel), actualUpdatedTagCategory)
    }

    @ParameterizedTest
    @MethodSource("readTagsData")
    fun testReadTags(
        tagIds: Set<TagId>,
        expectedTags: Set<TagCategoryModel>,
    ) {
        prepareMultiTagCategory()

        val actualTags = tagCategoryReadModelAdapter.readByTagIds(tagIds)

        assertEquals(expectedTags, actualTags)
    }

    @Test
    fun testExistMethods() {
        prepareMultiTagCategory()

        assertTrue(tagCategoryReadModelAdapter.tagCategoryExistsById(tagCategoryId1))
        assertTrue(tagCategoryReadModelAdapter.tagCategoryExistsById(tagCategoryId2))
        assertTrue(tagCategoryReadModelAdapter.tagCategoryExistsByName(tagCategoryName1))
        assertTrue(tagCategoryReadModelAdapter.tagCategoryExistsByName(tagCategoryName2))
        assertTrue(tagCategoryReadModelAdapter.tagExistsById(tagId1))
        assertTrue(tagCategoryReadModelAdapter.tagExistsById(tagId2))
        assertTrue(tagCategoryReadModelAdapter.tagExistsById(tagId3))
        assertTrue(tagCategoryReadModelAdapter.tagExistsByName(tagName1))
        assertTrue(tagCategoryReadModelAdapter.tagExistsByName(tagName2))
        assertTrue(tagCategoryReadModelAdapter.tagExistsByName(tagName3))

        assertFalse(
            tagCategoryReadModelAdapter.tagCategoryExistsById(
                TagCategoryId.fromString("cfd18367-5caa-4c5d-94d3-5f9ddc9a1820"),
            ),
        )
        assertFalse(tagCategoryReadModelAdapter.tagCategoryExistsByName("nonexistent tag category name"))
        assertFalse(
            tagCategoryReadModelAdapter.tagExistsById(
                TagId.fromString("40ecdbf8-18e8-4f54-819b-9e73d726e10c"),
            ),
        )
        assertFalse(tagCategoryReadModelAdapter.tagExistsByName("nonexistent tag name"))
    }

    private fun prepareMultiTagCategory() {
        tagCategoryRepository.saveAll(
            listOf(
                TagCategoryMapper.toDocument(tagCategory1MultipleTagModel),
                TagCategoryMapper.toDocument(tagCategory2Model),
            ),
        )
    }
}

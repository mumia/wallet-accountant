package com.wa.walletaccountant.domain.tagcategory

import com.ninjasquad.springmockk.MockkBean
import com.wa.walletaccountant.application.interceptor.command.tagcategory.AddNewTagToExistingCategoryCommandInterceptor
import com.wa.walletaccountant.application.interceptor.command.tagcategory.AddNewTagToNewCategoryCommandInterceptor
import com.wa.walletaccountant.application.interceptor.service.TagCategoryValidator
import com.wa.walletaccountant.application.interceptor.service.TagValidator
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToExistingCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToNewCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToExistingCategoryEvent
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToNewCategoryEvent
import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import io.mockk.Runs
import io.mockk.every
import io.mockk.just
import io.mockk.verify
import org.axonframework.test.aggregate.AggregateTestFixture
import org.axonframework.test.aggregate.FixtureConfiguration
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.boot.test.context.SpringBootTest

@SpringBootTest(
    classes = [
        TagCategoryReadModelPort::class,
        AddNewTagToNewCategoryCommandInterceptor::class,
        AddNewTagToExistingCategoryCommandInterceptor::class,
        TagValidator::class,
        TagCategoryValidator::class,
    ],
)
class TagCategoryAggregateTest
    @Autowired
    constructor(
        val addNewTagToNewCategoryCommandInterceptor: AddNewTagToNewCategoryCommandInterceptor,
        val addNewTagToExistingCategoryCommandInterceptor: AddNewTagToExistingCategoryCommandInterceptor,
    ) {
        companion object {
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
            private val tagNotes = "tag notes"

            val tag1 =
                Tag(
                    tagId = tagId1,
                    name = tagName1,
                    notes = null,
                )

            val tag2 =
                Tag(
                    tagId = tagId2,
                    name = tagName2,
                    notes = tagNotes,
                )

            val addNewTagToNewCategoryCommand =
                AddNewTagToNewCategoryCommand(
                    tagCategoryId = tagCategoryId1,
                    name = tagCategoryName1,
                    notes = null,
                    newTag = tag1,
                )
        }

        @MockkBean
        lateinit var tagCategoryValidator: TagCategoryValidator

        @MockkBean
        lateinit var tagValidator: TagValidator

        lateinit var fixture: FixtureConfiguration<TagCategoryAggregate>

        @BeforeEach
        fun setUp() {
            fixture =
                AggregateTestFixture(TagCategoryAggregate::class.java)
                    .registerCommandDispatchInterceptor(addNewTagToNewCategoryCommandInterceptor)
                    .registerCommandDispatchInterceptor(addNewTagToExistingCategoryCommandInterceptor)
        }

        @Test
        fun testAddTagToNewTagCategorySuccessfully() {
            val event =
                NewTagAddedToNewCategoryEvent(
                    tagCategoryId = tagCategoryId1,
                    name = tagCategoryName1,
                    notes = null,
                    tag = tag1,
                )

            every { tagCategoryValidator.validateTagCategoryCanBeAdded(any(), any()) } just Runs
            every { tagValidator.validateTagCanBeAdded(any(), any()) } just Runs

            fixture
                .givenNoPriorActivity()
                .`when`(addNewTagToNewCategoryCommand)
                .expectSuccessfulHandlerExecution()
                .expectEvents(event)

            verify(atMost = 1) { tagCategoryValidator.validateTagCategoryCanBeAdded(tagCategoryId1, tagCategoryName1) }
            verify(atMost = 1) { tagValidator.validateTagCanBeAdded(tagId1, tagName1) }
        }

        @Test
        fun testAddTagToExistingTagCategorySuccessfully() {
            val command =
                AddNewTagToExistingCategoryCommand(
                    tagCategoryId = tagCategoryId1,
                    newTag = tag2,
                )

            val event =
                NewTagAddedToExistingCategoryEvent(
                    tagCategoryId = tagCategoryId1,
                    tag = tag2,
                )

            every { tagCategoryValidator.validateTagCategoryCanBeAdded(any(), any()) } just Runs
            every { tagCategoryValidator.validateTagCategoryIdExists(any()) } just Runs
            every { tagValidator.validateTagCanBeAdded(any(), any()) } just Runs

            fixture
                .givenCommands(addNewTagToNewCategoryCommand)
                .`when`(command)
                .expectSuccessfulHandlerExecution()
                .expectEvents(event)

            verify(atMost = 1) { tagCategoryValidator.validateTagCategoryCanBeAdded(tagCategoryId1, tagCategoryName1) }
            verify(atMost = 1) { tagCategoryValidator.validateTagCategoryIdExists(tagCategoryId1) }
            verify(atMost = 1) { tagValidator.validateTagCanBeAdded(tagId2, tagName2) }
        }
    }

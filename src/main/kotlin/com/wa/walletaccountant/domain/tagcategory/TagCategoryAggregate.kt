package com.wa.walletaccountant.domain.tagcategory

import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToExistingCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToNewCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToExistingCategoryEvent
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToNewCategoryEvent
import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.axonframework.commandhandling.CommandHandler
import org.axonframework.eventsourcing.EventSourcingHandler
import org.axonframework.extensions.kotlin.applyEvent
import org.axonframework.modelling.command.AggregateCreationPolicy
import org.axonframework.modelling.command.AggregateIdentifier
import org.axonframework.modelling.command.CreationPolicy
import org.axonframework.spring.stereotype.Aggregate

@Aggregate
class TagCategoryAggregate {
    @AggregateIdentifier
    private var aggregateId: TagCategoryId? = null
    private val tags: MutableMap<TagId, Tag> = mutableMapOf()

    @CommandHandler
    @CreationPolicy(AggregateCreationPolicy.ALWAYS)
    fun on(command: AddNewTagToNewCategoryCommand) {
        applyEvent(
            NewTagAddedToNewCategoryEvent(
                tagCategoryId = command.tagCategoryId,
                name = command.name,
                notes = command.notes,
                tag = command.newTag,
            ),
        )
    }

    @CommandHandler
    fun on(command: AddNewTagToExistingCategoryCommand) {
        applyEvent(
            NewTagAddedToExistingCategoryEvent(
                tagCategoryId = command.tagCategoryId,
                tag = command.newTag,
            ),
        )
    }

    @EventSourcingHandler
    fun on(event: NewTagAddedToNewCategoryEvent) {
        aggregateId = event.tagCategoryId
        tags[event.tag.tagId] = event.tag
    }

    @EventSourcingHandler
    fun on(event: NewTagAddedToExistingCategoryEvent) {
        tags[event.tag.tagId] = event.tag
    }
}

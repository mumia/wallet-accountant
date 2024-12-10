package com.wa.walletaccountant.application.projection.tagcategory

import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.model.tagcategory.TagModel
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToExistingCategoryEvent
import com.wa.walletaccountant.domain.tagcategory.event.NewTagAddedToNewCategoryEvent
import org.axonframework.config.ProcessingGroup
import org.axonframework.eventhandling.EventHandler
import org.springframework.stereotype.Component

@Component
@ProcessingGroup("tag-category-read-model")
class TagCategoryReadModelProjection(
    private val readModelPort: TagCategoryReadModelPort,
) {
    @EventHandler
    fun on(event: NewTagAddedToNewCategoryEvent) {
        readModelPort.addNewTagToNewCategory(
            TagCategoryModel(
                tagCategoryId = event.tagCategoryId,
                name = event.name,
                notes = event.notes,
                tags =
                    setOf(
                        TagModel(
                            tagId = event.tag.tagId,
                            name = event.tag.name,
                            notes = event.tag.notes,
                        ),
                    ),
            ),
        )
    }

    @EventHandler
    fun on(event: NewTagAddedToExistingCategoryEvent) {
        readModelPort.addNewTagToExistingCategory(
            event.tagCategoryId,
            TagModel(
                tagId = event.tag.tagId,
                name = event.tag.name,
                notes = event.tag.notes,
            ),
        )
    }
}

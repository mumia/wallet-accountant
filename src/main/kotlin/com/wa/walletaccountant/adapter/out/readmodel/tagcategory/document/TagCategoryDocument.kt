package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import org.springframework.data.annotation.TypeAlias
import org.springframework.data.mongodb.core.mapping.Document
import org.springframework.data.mongodb.core.mapping.MongoId

@Document("tagCategory")
@TypeAlias("TagCategory")
data class TagCategoryDocument(
    @MongoId
    val tagCategoryId: TagCategoryId,
    val name: String,
    val notes: String?,
    val tags: List<TagDocument>,
) {
    fun addNewTag(newTag: TagDocument): TagCategoryDocument {
        val newTags = tags.toMutableList()
        newTags.add(newTag)

        return TagCategoryDocument(
            tagCategoryId = tagCategoryId,
            name = name,
            notes = notes,
            tags = newTags.toList(),
        )
    }
}

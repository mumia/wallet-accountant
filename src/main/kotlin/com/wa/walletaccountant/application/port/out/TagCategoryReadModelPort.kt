package com.wa.walletaccountant.application.port.out

import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.model.tagcategory.TagModel
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

interface TagCategoryReadModelPort {
    fun addNewTagToNewCategory(model: TagCategoryModel)

    fun addNewTagToExistingCategory(
        tagCategoryId: TagCategoryId,
        model: TagModel,
    )

    fun readAllTags(): Set<TagCategoryModel>

    fun readByTagIds(tagsIds: Set<TagId>): Set<TagCategoryModel>

    fun tagExistsById(tagId: TagId): Boolean

    fun tagExistsByName(tagName: String): Boolean

    fun tagCategoryExistsById(tagCategoryId: TagCategoryId): Boolean

    fun tagCategoryExistsByName(tagCategoryName: String): Boolean
}

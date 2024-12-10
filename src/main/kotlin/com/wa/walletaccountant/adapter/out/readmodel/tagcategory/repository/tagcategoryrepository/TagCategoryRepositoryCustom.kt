package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.tagcategoryrepository

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document.TagCategoryDocument
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

interface TagCategoryRepositoryCustom {
    fun findByTags(tagIds: List<TagId>): List<TagCategoryDocument>

    fun tagExistsById(tagId: TagId): Boolean

    fun tagExistsByName(name: String): Boolean

    fun tagCategoryExistsByName(name: String): Boolean
}

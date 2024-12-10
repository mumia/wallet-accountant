package com.wa.walletaccountant.application.model.tagcategory

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId

data class TagCategoryModel(
    val tagCategoryId: TagCategoryId,
    val name: String,
    val notes: String?,
    val tags: Set<TagModel>,
)

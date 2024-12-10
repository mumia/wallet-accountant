package com.wa.walletaccountant.domain.tagcategory.event

import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId

data class NewTagAddedToNewCategoryEvent(
    val tagCategoryId: TagCategoryId,
    val name: String,
    val notes: String?,
    val tag: Tag,
)

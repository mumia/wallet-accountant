package com.wa.walletaccountant.domain.tagcategory.event

import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId

data class NewTagAddedToExistingCategoryEvent(
    val tagCategoryId: TagCategoryId,
    val tag: Tag,
)

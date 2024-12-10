package com.wa.walletaccountant.domain.tagcategory.tagcategory

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class Tag(
    val tagId: TagId,
    val name: String,
    val notes: String?,
)

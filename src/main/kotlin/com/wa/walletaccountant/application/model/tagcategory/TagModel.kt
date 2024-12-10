package com.wa.walletaccountant.application.model.tagcategory

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class TagModel(
    val tagId: TagId,
    val name: String,
    val notes: String?,
)

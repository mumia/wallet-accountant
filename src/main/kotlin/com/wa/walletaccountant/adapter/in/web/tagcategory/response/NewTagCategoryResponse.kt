package com.wa.walletaccountant.adapter.`in`.web.tagcategory.response

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class NewTagCategoryResponse(
    val tagId: TagId,
    val tagCategoryId: TagCategoryId,
)

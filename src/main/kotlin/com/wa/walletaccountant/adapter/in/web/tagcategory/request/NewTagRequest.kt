package com.wa.walletaccountant.adapter.`in`.web.tagcategory.request

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import jakarta.validation.constraints.NotEmpty
import org.hibernate.validator.constraints.UUID

data class NewTagRequest(
    @NotEmpty @UUID val tagCategoryId: TagCategoryId,
    @NotEmpty val tagName: String,
    val tagNotes: String?,
)

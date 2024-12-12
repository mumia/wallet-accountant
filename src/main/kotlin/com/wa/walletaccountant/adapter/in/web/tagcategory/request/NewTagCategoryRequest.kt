package com.wa.walletaccountant.adapter.`in`.web.tagcategory.request

import jakarta.validation.constraints.NotEmpty

data class NewTagCategoryRequest(
    @NotEmpty val categoryName: String,
    val categoryNotes: String?,
    @NotEmpty val tagName: String,
    val tagNotes: String?,
)

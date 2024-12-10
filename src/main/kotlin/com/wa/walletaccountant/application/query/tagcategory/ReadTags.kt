package com.wa.walletaccountant.application.query.tagcategory

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

data class ReadTags(
    val filter: Set<TagId>,
)

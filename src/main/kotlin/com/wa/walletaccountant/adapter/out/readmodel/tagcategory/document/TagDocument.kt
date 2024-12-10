package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.data.mongodb.core.mapping.MongoId

data class TagDocument(
    @MongoId
    val tagId: TagId,
    val name: String,
    val notes: String?,
)

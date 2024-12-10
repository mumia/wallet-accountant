package com.wa.walletaccountant.application.interceptor.exception

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId

class UnknownTagCategoryException private constructor(
    identifier: String,
) : RuntimeException(
    "Unknown tag category. [%s]".format(identifier),
) {
    companion object {
        fun fromTagCategoryId(tagCategoryId: TagCategoryId): UnknownTagCategoryException =
            UnknownTagCategoryException(
                "TagCategoryId: %s".format(tagCategoryId.toString()),
            )
    }
}

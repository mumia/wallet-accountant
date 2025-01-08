package com.wa.walletaccountant.application.interceptor.exception

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

class UnknownTagException private constructor(
    identifier: String,
) : UnknownEntityException(
    "Unknown tag. [%s]".format(identifier),
) {
    companion object {
        fun fromTagId(tagId: TagId): UnknownTagException =
            UnknownTagException(
                "TagId: %s".format(tagId.toString()),
            )
    }
}

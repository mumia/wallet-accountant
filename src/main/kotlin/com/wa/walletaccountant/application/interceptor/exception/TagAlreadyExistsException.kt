package com.wa.walletaccountant.application.interceptor.exception

import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

class TagAlreadyExistsException private constructor(
    identifier: String,
) : RuntimeException(
    "Tag already exists. [%s]".format(identifier),
) {
    companion object {
        fun fromTagId(tagId: TagId): TagAlreadyExistsException =
            TagAlreadyExistsException("TagId: ".format(tagId.toString()))

        fun fromName(name: String): TagAlreadyExistsException =
            TagAlreadyExistsException("Name: %s".format(name))
    }
}

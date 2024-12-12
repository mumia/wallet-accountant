package com.wa.walletaccountant.application.interceptor.exception

import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId

class TagCategoryAlreadyExistsException private constructor(
    identifier: String,
) : RuntimeException(
    "Tag category already exists. [%s]".format(identifier),
) {
    companion object {
        fun fromTagCategoryId(tagCategoryId: TagCategoryId): TagCategoryAlreadyExistsException =
            TagCategoryAlreadyExistsException("TagCategoryId: ".format(tagCategoryId.toString()))

        fun fromName(name: String): TagCategoryAlreadyExistsException =
            TagCategoryAlreadyExistsException("Name: %s".format(name))
    }
}

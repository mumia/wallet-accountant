package com.wa.walletaccountant.application.interceptor.service

import com.wa.walletaccountant.application.interceptor.exception.TagAlreadyExistsException
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.stereotype.Component

@Component
class TagValidator(
    private val readModel: TagCategoryReadModelPort,
) {
    fun validateTagCanBeAdded(tagId: TagId, name: String) {
        if (readModel.tagExistsById(tagId)) {
            throw TagAlreadyExistsException.fromTagId(tagId)
        }

        if (readModel.tagExistsByName(name)) {
            throw TagAlreadyExistsException.fromName(name)
        }
    }
}
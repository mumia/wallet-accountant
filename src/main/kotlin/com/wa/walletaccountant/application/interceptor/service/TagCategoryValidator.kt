package com.wa.walletaccountant.application.interceptor.service

import com.wa.walletaccountant.application.interceptor.exception.TagCategoryAlreadyExistsException
import com.wa.walletaccountant.application.interceptor.exception.UnknownTagCategoryException
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import org.springframework.stereotype.Component

@Component
class TagCategoryValidator(
    private val readModel: TagCategoryReadModelPort,
) {
    fun validateTagCategoryCanBeAdded(tagCategoryId: TagCategoryId, name: String) {
        if (readModel.tagCategoryExistsById(tagCategoryId)) {
            throw TagCategoryAlreadyExistsException.fromTagCategoryId(tagCategoryId)
        }

        if (readModel.tagCategoryExistsByName(name)) {
            throw TagCategoryAlreadyExistsException.fromName(name)
        }
    }

    fun validateTagCategoryIdExists(tagCategoryId: TagCategoryId) {
        if (!readModel.tagCategoryExistsById(tagCategoryId)) {
            throw UnknownTagCategoryException.fromTagCategoryId(tagCategoryId)
        }
    }
}

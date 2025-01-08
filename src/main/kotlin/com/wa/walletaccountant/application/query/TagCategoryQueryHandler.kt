package com.wa.walletaccountant.application.query

import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.application.query.tagcategory.ReadTags
import org.axonframework.queryhandling.QueryHandler
import org.springframework.stereotype.Component

@Component
class TagCategoryQueryHandler(
    private val tagCategoryReadModelPort: TagCategoryReadModelPort,
) {
    @QueryHandler
    fun on(readTags: ReadTags): Set<TagCategoryModel> {
        if (readTags.filter == null || readTags.filter.isEmpty()) {
            return tagCategoryReadModelPort.readAllTags()
        }

        return tagCategoryReadModelPort.readByTagIds(readTags.filter)
    }
}

package com.wa.walletaccountant.adapter.`in`.web.tagcategory.mapper

import com.wa.walletaccountant.adapter.`in`.web.tagcategory.request.NewTagCategoryRequest
import com.wa.walletaccountant.adapter.`in`.web.tagcategory.request.NewTagRequest
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToExistingCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToNewCategoryCommand
import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId

object TagCategoryMapper {
    fun toNewTagCategoryCommand(
        tagCategoryId: TagCategoryId,
        tagId: TagId,
        newTagCategoryRequest: NewTagCategoryRequest,
    ): AddNewTagToNewCategoryCommand =
        AddNewTagToNewCategoryCommand(
            tagCategoryId = tagCategoryId,
            name = newTagCategoryRequest.categoryName,
            notes = newTagCategoryRequest.categoryNotes,
            newTag =
                Tag(
                    tagId = tagId,
                    name = newTagCategoryRequest.tagName,
                    notes = newTagCategoryRequest.tagNotes,
                ),
        )

    fun toNewTagCommand(
        tagId: TagId,
        newTagRequest: NewTagRequest,
    ): AddNewTagToExistingCategoryCommand =
        AddNewTagToExistingCategoryCommand(
            tagCategoryId = newTagRequest.tagCategoryId,
            newTag =
                Tag(
                    tagId = tagId,
                    name = newTagRequest.tagName,
                    notes = newTagRequest.tagNotes,
                ),
        )
}

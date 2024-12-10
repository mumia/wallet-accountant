package com.wa.walletaccountant.domain.tagcategory.command

import com.wa.walletaccountant.domain.tagcategory.tagcategory.Tag
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import org.axonframework.modelling.command.TargetAggregateIdentifier

data class AddNewTagToExistingCategoryCommand(
    @TargetAggregateIdentifier
    val tagCategoryId: TagCategoryId,
    val newTag: Tag,
)

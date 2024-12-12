package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document.TagDocument
import com.wa.walletaccountant.application.model.tagcategory.TagModel

object TagMapper {
    fun toDocument(model: TagModel): TagDocument =
        TagDocument(
            tagId = model.tagId,
            name = model.name,
            notes = model.notes,
        )

    fun toModel(document: TagDocument): TagModel =
        TagModel(
            tagId = document.tagId,
            name = document.name,
            notes = document.notes,
        )
}

package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document.TagCategoryDocument
import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import java.util.stream.Collectors

object TagCategoryMapper {
    fun toDocument(model: TagCategoryModel): TagCategoryDocument =
        TagCategoryDocument(
            tagCategoryId = model.tagCategoryId,
            name = model.name,
            notes = model.notes,
            tags =
                model.tags
                    .stream()
                    .map { TagMapper.toDocument(it) }
                    .toList(),
        )

    fun toModel(document: TagCategoryDocument): TagCategoryModel = toModel(document, emptySet())

    fun toModel(
        document: TagCategoryDocument,
        tagIds: Set<TagId>,
    ): TagCategoryModel =
        TagCategoryModel(
            tagCategoryId = document.tagCategoryId,
            name = document.name,
            notes = document.notes,
            tags =
                document.tags
                    .stream()
                    .filter { tagIds.isEmpty() || tagIds.contains(it.tagId) }
                    .map { TagMapper.toModel(it) }
                    .collect(Collectors.toSet()),
        )
}

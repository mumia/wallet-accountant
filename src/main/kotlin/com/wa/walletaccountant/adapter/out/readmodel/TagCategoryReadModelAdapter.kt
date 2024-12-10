package com.wa.walletaccountant.adapter.out.readmodel

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper.TagCategoryMapper
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.mapper.TagMapper
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.TagCategoryRepository
import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.model.tagcategory.TagModel
import com.wa.walletaccountant.application.port.out.TagCategoryReadModelPort
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.stereotype.Component
import java.util.stream.Collectors

@Component
class TagCategoryReadModelAdapter(
    private val tagCategoryRepository: TagCategoryRepository,
) : TagCategoryReadModelPort {
    override fun addNewTagToNewCategory(model: TagCategoryModel) {
        tagCategoryRepository.save(TagCategoryMapper.toDocument(model))
    }

    override fun addNewTagToExistingCategory(
        tagCategoryId: TagCategoryId,
        model: TagModel,
    ) {
        val tagCategoryOptional = tagCategoryRepository.findById(tagCategoryId)

        if (tagCategoryOptional.isEmpty) {
            return
        }

        val newTagCategory = tagCategoryOptional.get().addNewTag(TagMapper.toDocument(model))

        tagCategoryRepository.save(newTagCategory)
    }

    override fun readAllTags(): Set<TagCategoryModel> =
        tagCategoryRepository
            .findAll()
            .stream()
            .map { TagCategoryMapper.toModel(it) }
            .collect(Collectors.toSet())

    override fun readByTagIds(tagsIds: Set<TagId>): Set<TagCategoryModel> {
        if (tagsIds.isEmpty()) {
            return readAllTags()
        }

        return tagCategoryRepository
            .findByTags(tagIds = tagsIds.toList())
            .stream()
            .map { TagCategoryMapper.toModel(it, tagsIds) }
            .collect(Collectors.toSet())
    }

    override fun tagExistsById(tagId: TagId): Boolean = tagCategoryRepository.tagExistsById(tagId)

    override fun tagExistsByName(tagName: String): Boolean = tagCategoryRepository.tagExistsByName(tagName)

    override fun tagCategoryExistsById(tagCategoryId: TagCategoryId): Boolean =
        tagCategoryRepository.existsById(
            tagCategoryId,
        )

    override fun tagCategoryExistsByName(tagCategoryName: String): Boolean =
        tagCategoryRepository.tagCategoryExistsByName(
            tagCategoryName,
        )
}

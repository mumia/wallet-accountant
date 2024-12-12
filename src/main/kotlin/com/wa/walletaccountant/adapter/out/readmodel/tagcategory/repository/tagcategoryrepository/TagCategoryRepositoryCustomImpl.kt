package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.tagcategoryrepository

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document.TagCategoryDocument
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.data.mongodb.core.MongoTemplate
import org.springframework.data.mongodb.core.query.Criteria
import org.springframework.data.mongodb.core.query.Query
import org.springframework.stereotype.Component

@Component
class TagCategoryRepositoryCustomImpl
@Autowired
constructor(
    private val mongoTemplate: MongoTemplate,
) : TagCategoryRepositoryCustom {
    override fun findByTags(tagIds: List<TagId>): List<TagCategoryDocument> {
        val query = Query()

        query.addCriteria(Criteria.where("tags.tagId").`in`(tagIds))

        return mongoTemplate.find(query, TagCategoryDocument::class.java)
    }

    override fun tagExistsById(tagId: TagId): Boolean = exists(Criteria.where("tags.tagId").`is`(tagId))

    override fun tagExistsByName(name: String): Boolean = exists(Criteria.where("tags.name").`is`(name))

    override fun tagCategoryExistsByName(name: String): Boolean = exists(Criteria.where("name").`is`(name))

    private fun exists(criteria: Criteria): Boolean {
        val query = Query()

        query.addCriteria(criteria)

        return mongoTemplate.exists(query, TagCategoryDocument::class.java)
    }
}

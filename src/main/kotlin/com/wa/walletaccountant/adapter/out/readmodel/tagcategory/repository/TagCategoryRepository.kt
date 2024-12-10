package com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository

import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.document.TagCategoryDocument
import com.wa.walletaccountant.adapter.out.readmodel.tagcategory.repository.tagcategoryrepository.TagCategoryRepositoryCustom
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import org.springframework.data.mongodb.repository.MongoRepository

interface TagCategoryRepository :
    MongoRepository<TagCategoryDocument, TagCategoryId>,
    TagCategoryRepositoryCustom

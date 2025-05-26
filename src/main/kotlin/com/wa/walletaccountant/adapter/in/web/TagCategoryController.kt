package com.wa.walletaccountant.adapter.`in`.web

import com.wa.walletaccountant.adapter.`in`.web.tagcategory.mapper.TagCategoryMapper
import com.wa.walletaccountant.adapter.`in`.web.tagcategory.request.NewTagCategoryRequest
import com.wa.walletaccountant.adapter.`in`.web.tagcategory.request.NewTagRequest
import com.wa.walletaccountant.adapter.`in`.web.tagcategory.response.NewTagCategoryResponse
import com.wa.walletaccountant.adapter.`in`.web.tagcategory.response.NewTagResponse
import com.wa.walletaccountant.application.model.tagcategory.TagCategoryModel
import com.wa.walletaccountant.application.query.tagcategory.ReadTags
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.tagcategory.tagcategory.TagCategoryId
import com.wa.walletaccountant.domain.tagcategory.tagcategory.tag.TagId
import jakarta.validation.Valid
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.messaging.responsetypes.ResponseTypes
import org.axonframework.queryhandling.QueryGateway
import org.springframework.http.HttpStatus.OK
import org.springframework.http.ResponseEntity
import org.springframework.validation.annotation.Validated
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RequestParam
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController
import java.util.concurrent.CompletableFuture

@Validated
@RestController
@RequestMapping("/api/v1")
class TagCategoryController(
    private val commandGateway: CommandGateway,
    private val queryGateway: QueryGateway,
    private val idGenerator: IdGenerator,
) {
    @PostMapping(path = ["/tag-category"], consumes = ["application/json"], produces = ["application/json"])
    @ResponseStatus(OK)
    fun newTagCategory(
        @RequestBody @Valid request: NewTagCategoryRequest,
    ): CompletableFuture<ResponseEntity<NewTagCategoryResponse>> {
        val tagCategoryId = TagCategoryId(idGenerator.newId())
        val tagId = TagId(idGenerator.newId())

        return commandGateway
            .send<TagCategoryId>(TagCategoryMapper.toNewTagCategoryCommand(tagCategoryId, tagId, request))
            .thenApply { _ -> return@thenApply ResponseEntity.ok(NewTagCategoryResponse(tagId, tagCategoryId)) }
    }

    @PostMapping(path = ["/tag"], consumes = ["application/json"], produces = ["application/json"])
    @ResponseStatus(OK)
    fun newTag(
        @RequestBody @Valid request: NewTagRequest,
    ): CompletableFuture<ResponseEntity<NewTagResponse>> {
        val tagId = TagId(idGenerator.newId())

        return commandGateway
            .send<TagCategoryId>(TagCategoryMapper.toNewTagCommand(tagId, request))
            .thenApply { _ -> return@thenApply ResponseEntity.ok(NewTagResponse(tagId)) }
    }

    @GetMapping(path = ["/tags" ], produces = ["application/json"])
    @ResponseStatus(OK)
    fun readAllAccounts(
        @RequestParam(name = "filters[]", required = false) filters: Set<String>?,
    ): CompletableFuture<MutableList<TagCategoryModel>> =
        queryGateway.query(
            ReadTags(filters?.map { TagId.fromString(it) }?.toSet()),
            ResponseTypes.multipleInstancesOf(TagCategoryModel::class.java),
        )
}

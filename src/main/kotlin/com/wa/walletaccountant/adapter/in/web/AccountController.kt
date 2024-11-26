package com.wa.walletaccountant.adapter.`in`.web

import com.wa.walletaccountant.adapter.`in`.web.account.mapper.AccountMapper
import com.wa.walletaccountant.adapter.`in`.web.account.request.NewAccountRequest
import com.wa.walletaccountant.adapter.`in`.web.account.response.NewAccountResponse
import com.wa.walletaccountant.application.model.account.AccountModel
import com.wa.walletaccountant.application.query.account.ReadAccountById
import com.wa.walletaccountant.application.query.account.ReadAllAccounts
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import jakarta.validation.Valid
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.messaging.responsetypes.ResponseTypes
import org.axonframework.queryhandling.QueryGateway
import org.hibernate.validator.constraints.UUID
import org.springframework.http.HttpStatus
import org.springframework.http.ResponseEntity
import org.springframework.scheduling.annotation.Async
import org.springframework.validation.annotation.Validated
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController
import java.util.concurrent.CompletableFuture

@Validated
@RestController
@RequestMapping("/api/v1")
class AccountController(
    private val commandGateway: CommandGateway,
    private val queryGateway: QueryGateway,
    private val idGenerator: IdGenerator,
) {
    @PostMapping(value = ["account"], consumes = ["application/json"], produces = ["application/json"])
    @ResponseStatus(HttpStatus.CREATED)
    @Async
    fun registerAccount(
        @RequestBody @Valid request: NewAccountRequest,
    ): CompletableFuture<ResponseEntity<NewAccountResponse>> {
        val accountId = AccountId(idGenerator.newId())

        return commandGateway
            .send<AccountId>(AccountMapper.toCommand(accountId, request))
            .thenApply { _ -> return@thenApply ResponseEntity.ok(NewAccountResponse(accountId)) }
    }

    @GetMapping(path = ["account/{accountId}"], produces = ["application/json"])
    fun readAccount(
        @PathVariable @UUID(message = "Invalid account id, must be a valid UUID") accountId: String,
    ): CompletableFuture<ResponseEntity<AccountModel>> =
        queryGateway
            .query(
                ReadAccountById(AccountId.fromString(accountId)),
                ResponseTypes.optionalInstanceOf(AccountModel::class.java),
            ).thenApply { optional ->
                optional
                    .map { ResponseEntity.ok(it) }
                    .orElseGet { ResponseEntity.notFound().build() }
            }

    @GetMapping(path = ["accounts"], produces = ["application/json"])
    @ResponseStatus(HttpStatus.OK)
    fun readAllAccounts(): CompletableFuture<MutableList<AccountModel>>? =
        queryGateway.query(ReadAllAccounts(), ResponseTypes.multipleInstancesOf(AccountModel::class.java))
}

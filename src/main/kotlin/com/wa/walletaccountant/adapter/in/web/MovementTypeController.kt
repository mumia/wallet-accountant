package com.wa.walletaccountant.adapter.`in`.web

import com.wa.walletaccountant.adapter.`in`.web.movementtype.mapper.MovementTypeMapper
import com.wa.walletaccountant.adapter.`in`.web.movementtype.request.RegisterMovementTypeRequest
import com.wa.walletaccountant.adapter.`in`.web.movementtype.response.NewMovementRegisteredTypeResponse
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.application.query.movementtype.ReadAllMovementTypes
import com.wa.walletaccountant.application.query.movementtype.ReadMovementTypeById
import com.wa.walletaccountant.application.query.movementtype.ReadMovementTypesForAccount
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
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
class MovementTypeController(
    private val commandGateway: CommandGateway,
    private val queryGateway: QueryGateway,
    private val idGenerator: IdGenerator,
) {
    @PostMapping(value = ["movement-type"], consumes = ["application/json"], produces = ["application/json"])
    @ResponseStatus(HttpStatus.CREATED)
    @Async
    fun registerAccount(
        @RequestBody @Valid request: RegisterMovementTypeRequest,
    ): CompletableFuture<ResponseEntity<NewMovementRegisteredTypeResponse>> {
        val movementTypeId = MovementTypeId(idGenerator.newId())

        return commandGateway
            .send<MovementTypeId>(MovementTypeMapper.toCommand(movementTypeId, request))
            .thenApply { _ -> return@thenApply ResponseEntity.ok(NewMovementRegisteredTypeResponse(movementTypeId)) }
    }

    @GetMapping(path = ["movement-type/{movementTypeId}"], produces = ["application/json"])
    fun readMovementType(
        @PathVariable @UUID(message = "Invalid movement type id, must be a valid UUID") movementTypeId: String,
    ): CompletableFuture<ResponseEntity<MovementTypeModel>> =
        queryGateway
            .query(
                ReadMovementTypeById(MovementTypeId.fromString(movementTypeId)),
                ResponseTypes.optionalInstanceOf(MovementTypeModel::class.java),
            ).thenApply { optional ->
                optional
                    .map { ResponseEntity.ok(it) }
                    .orElseGet { ResponseEntity.notFound().build() }
            }

    @GetMapping(path = ["movement-types"], produces = ["application/json"])
    @ResponseStatus(HttpStatus.OK)
    fun readAllMovementTypes(): CompletableFuture<MutableList<MovementTypeModel>>? =
        queryGateway.query(ReadAllMovementTypes(), ResponseTypes.multipleInstancesOf(MovementTypeModel::class.java))

    @GetMapping(path = ["movement-types/account/{accountId}"], produces = ["application/json"])
    @ResponseStatus(HttpStatus.OK)
    fun readMovementTypesForAccount(
        @PathVariable @UUID(message = "Invalid account id, must be a valid UUID") accountId: String,
    ): CompletableFuture<MutableList<MovementTypeModel>>? =
        queryGateway.query(
            ReadMovementTypesForAccount(AccountId.fromString(accountId)),
            ResponseTypes.multipleInstancesOf(MovementTypeModel::class.java)
        )
}

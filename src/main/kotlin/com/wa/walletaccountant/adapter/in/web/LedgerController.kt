package com.wa.walletaccountant.adapter.`in`.web

import com.wa.walletaccountant.adapter.`in`.web.ledger.mapper.LedgerMapper
import com.wa.walletaccountant.adapter.`in`.web.ledger.request.RegisterTransactionRequest
import com.wa.walletaccountant.adapter.`in`.web.ledger.request.TransactionRegisteredResponse
import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.query.ledger.ReadCurrentMonthLedger
import com.wa.walletaccountant.common.IdGenerator
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.ledger.ledger.TransactionId
import jakarta.validation.Valid
import org.axonframework.commandhandling.gateway.CommandGateway
import org.axonframework.messaging.responsetypes.ResponseTypes
import org.axonframework.queryhandling.QueryGateway
import org.hibernate.validator.constraints.UUID
import org.springframework.http.ResponseEntity
import org.springframework.validation.annotation.Validated
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.PostMapping
import org.springframework.web.bind.annotation.RequestBody
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.RestController
import java.util.concurrent.CompletableFuture

@Validated
@RestController
@RequestMapping("/api/v1")
class LedgerController(
    private val commandGateway: CommandGateway,
    private val queryGateway: QueryGateway,
    private val idGenerator: IdGenerator,
) {
    @PostMapping(path = ["ledger/transaction"], consumes = ["application/json"])
    fun registerTransaction(
        @RequestBody @Valid request: RegisterTransactionRequest
    ): CompletableFuture<ResponseEntity<TransactionRegisteredResponse>> {
        val transactionId = TransactionId(idGenerator.newId())

        return commandGateway
            .send<TransactionId>(LedgerMapper.toCommand(transactionId, request))
            .thenApply { _ -> return@thenApply ResponseEntity.ok(TransactionRegisteredResponse(transactionId)) }
    }

    @GetMapping(path = ["ledger/{accountId}"], produces = ["application/json"])
    fun readCurrentMonthLedger(
        @PathVariable @UUID(message = "Invalid account id, must be a valid UUID") accountId: String,
    ): CompletableFuture<ResponseEntity<LedgerMonthModel>> =
        queryGateway
            .query(
                ReadCurrentMonthLedger(AccountId.fromString(accountId)),
                ResponseTypes.optionalInstanceOf(LedgerMonthModel::class.java),
            ).thenApply { optional ->
                optional
                    .map { ResponseEntity.ok(it) }
                    .orElseGet { ResponseEntity.notFound().build() }
            }
}

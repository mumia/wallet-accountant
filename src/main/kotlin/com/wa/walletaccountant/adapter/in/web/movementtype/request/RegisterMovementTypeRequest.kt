package com.wa.walletaccountant.adapter.`in`.web.movementtype.request

import com.wa.walletaccountant.adapter.`in`.web.constraint.EnumConstraint
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementAction
import jakarta.validation.constraints.NotEmpty
import org.hibernate.validator.constraints.UUID

data class RegisterMovementTypeRequest(
    @field:NotEmpty
    @field:EnumConstraint(enumClass = MovementAction::class)
    val action: String,
    @field:NotEmpty
    @field:UUID
    val accountId: String,
    @field:UUID(allowEmpty = true)
    val sourceAccountId: String?,
    @field:NotEmpty
    val description: String,
    val notes: String?,
    @field:NotEmpty
    val tagIds: Set<String>
)
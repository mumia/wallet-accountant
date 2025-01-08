package com.wa.walletaccountant.application.query.movementtype

import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId

data class ReadMovementTypeById(
    val movementTypeId: MovementTypeId,
) {
    override fun toString(): String = movementTypeId.toString()
}

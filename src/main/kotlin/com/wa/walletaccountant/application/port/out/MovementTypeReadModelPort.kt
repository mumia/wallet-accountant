package com.wa.walletaccountant.application.port.out

import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import java.util.Optional

interface MovementTypeReadModelPort {
    fun registerNewMovementType(model: MovementTypeModel)

    fun readMovementType(movementTypeId: MovementTypeId): Optional<MovementTypeModel>

    fun readAllMovementTypes(): Set<MovementTypeModel>
}

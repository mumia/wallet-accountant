package com.wa.walletaccountant.application.query

import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.application.port.out.MovementTypeReadModelPort
import com.wa.walletaccountant.application.query.movementtype.ReadAllMovementTypes
import com.wa.walletaccountant.application.query.movementtype.ReadMovementTypeById
import org.axonframework.queryhandling.QueryHandler
import org.springframework.stereotype.Component
import java.util.Optional

@Component
class MovementTypeQueryHandler(
    private val readModel: MovementTypeReadModelPort,
) {
    @QueryHandler
    fun readMovementType(query: ReadMovementTypeById): Optional<MovementTypeModel> =
        readModel.readMovementType(query.movementTypeId)

    @QueryHandler
    fun readAllMovementTypes(query: ReadAllMovementTypes): Set<MovementTypeModel> = readModel.readAllMovementTypes()
}

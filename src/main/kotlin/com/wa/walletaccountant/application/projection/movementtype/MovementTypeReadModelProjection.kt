package com.wa.walletaccountant.application.projection.movementtype

import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.application.port.out.MovementTypeReadModelPort
import com.wa.walletaccountant.domain.movementtype.event.NewMovementTypeRegisteredEvent
import org.axonframework.config.ProcessingGroup
import org.axonframework.eventhandling.EventHandler
import org.springframework.stereotype.Component

@Component
@ProcessingGroup("movement-type-read-model")
class MovementTypeReadModelProjection(
    private val readModelPort: MovementTypeReadModelPort,
) {
    @EventHandler
    fun on(event: NewMovementTypeRegisteredEvent) {
        readModelPort.registerNewMovementType(
            MovementTypeModel(
                event.movementTypeId,
                event.movementAction,
                event.accountId,
                event.sourceAccountId,
                event.description,
                event.notes,
                event.tagIds
            ),
        )
    }
}

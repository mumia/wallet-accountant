package com.wa.walletaccountant.adapter.out.readmodel

import com.wa.walletaccountant.adapter.out.readmodel.movementtype.mapper.MovementTypeMapper
import com.wa.walletaccountant.adapter.out.readmodel.movementtype.repository.MovementTypeRepository
import com.wa.walletaccountant.application.model.movementtype.MovementTypeModel
import com.wa.walletaccountant.application.port.out.MovementTypeReadModelPort
import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import org.springframework.stereotype.Service
import java.util.Optional

@Service
class MovementTypeReadModelAdapter(
    private val movementTypeRepository: MovementTypeRepository,
) : MovementTypeReadModelPort {
    override fun registerNewMovementType(model: MovementTypeModel) {
        movementTypeRepository.save(MovementTypeMapper.toDocument(model))
    }

    override fun readMovementType(movementTypeId: MovementTypeId): Optional<MovementTypeModel> {
        val optionalMovementType = movementTypeRepository.findById(movementTypeId.id())
        if (optionalMovementType.isEmpty) {
            return Optional.empty()
        }

        return Optional.of(MovementTypeMapper.toModel(optionalMovementType.get()))
    }

    override fun readAllMovementTypes(): Set<MovementTypeModel> =
        movementTypeRepository
            .findAll()
            .map { MovementTypeMapper.toModel(it) }
            .toSet()

    override fun readMovementTypesForAccount(accountId: AccountId): Set<MovementTypeModel> =
        movementTypeRepository
            .findByAccountId(accountId)
            .map { MovementTypeMapper.toModel(it) }
            .toSet()
}

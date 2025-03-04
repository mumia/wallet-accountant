package com.wa.walletaccountant.adapter.out.readmodel.movementtype.repository

import com.wa.walletaccountant.adapter.out.readmodel.movementtype.document.MovementTypeDocument
import com.wa.walletaccountant.domain.movementtype.movementtype.MovementTypeId
import org.springframework.data.mongodb.repository.MongoRepository

interface MovementTypeRepository: MongoRepository<MovementTypeDocument, MovementTypeId>

package com.wa.walletaccountant.common

import org.springframework.stereotype.Component
import java.util.UUID

@Component
class IdGenerator {
    fun newId(): UUID = UUID.randomUUID()
}

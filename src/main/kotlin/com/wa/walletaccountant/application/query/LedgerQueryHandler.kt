package com.wa.walletaccountant.application.query

import com.wa.walletaccountant.application.model.ledger.LedgerMonthModel
import com.wa.walletaccountant.application.port.out.LedgerReadModelPort
import com.wa.walletaccountant.application.query.account.ReadAccountById
import org.axonframework.queryhandling.QueryHandler
import org.springframework.stereotype.Component
import java.util.Optional

@Component
class LedgerQueryHandler(
    private val readModel: LedgerReadModelPort,
) {
    @QueryHandler
    fun readAccountById(query: ReadAccountById): Optional<LedgerMonthModel> =
        readModel.readCurrentMonthLedger(query.accountId)
}

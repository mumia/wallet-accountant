package com.wa.walletaccountant.domain.ledger.exception

import com.wa.walletaccountant.domain.common.Money
import com.wa.walletaccountant.domain.exception.AggregateLogicException
import com.wa.walletaccountant.domain.ledger.ledger.LedgerId

class CloseBalanceDoesNotMatchCurrentBalanceException(
    ledgerId: LedgerId,
    currentBalance: Money,
    endBalance: Money,
) : AggregateLogicException(
    "Ledger",
    "Ledger balance does not match expected end of month balance. [ledgerId: ${ledgerId.id()}] " +
            "[accountId: ${ledgerId.accountId}] [month: ${ledgerId.month.value}] [year: ${ledgerId.year.value}] " +
            "[currentBalance: ${currentBalance}] [endBalance: ${endBalance}]"
)
package com.wa.walletaccountant.domain.common.exception

import com.wa.walletaccountant.domain.common.Currency
import com.wa.walletaccountant.domain.exception.DomainLogicException

class MismatchedMoneyCurrencyException(current: Currency, given: Currency) : DomainLogicException(
    "Mismatched money currency. [currentCurrency: %s] [givenCurrency: %s]".format(current.toString(), given.toString())
)

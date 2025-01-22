package com.wa.walletaccountant.domain.exception

abstract class AggregateLogicException(
    aggregate: String,
    message: String,
): DomainLogicException("%s: %s".format(aggregate, message))
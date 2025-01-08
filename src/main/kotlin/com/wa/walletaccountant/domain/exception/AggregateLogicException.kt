package com.wa.walletaccountant.domain.exception

abstract class AggregateLogicException(
    aggregate: String,
    message: String,
): RuntimeException("%s: %s".format(aggregate, message))
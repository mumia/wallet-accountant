package com.wa.walletaccountant.application.saga.exception

class SagaEventHandlingTimedOut(sagaName: String, eventName: String) : RuntimeException(
    "Saga timed out while handling event. [Saga: ${sagaName}] [Event: ${eventName}]"
)

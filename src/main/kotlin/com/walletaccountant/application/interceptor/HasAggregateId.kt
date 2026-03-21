package com.walletaccountant.application.interceptor

interface HasAggregateId<T> {
    fun withNewId(): T
}

package com.wa.walletaccountant.application.port.`in`

interface ProjectionManagerPort {
    fun getAllProjectors(): List<String>

    fun restartProjector(projectorName: String)
}
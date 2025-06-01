package com.wa.walletaccountant

import org.springframework.boot.autoconfigure.SpringBootApplication
import org.springframework.boot.runApplication
import org.springframework.scheduling.annotation.EnableAsync

@SpringBootApplication
@EnableAsync
class WalletAccountantApplication

fun main(args: Array<String>) {
    runApplication<WalletAccountantApplication>(*args)
}

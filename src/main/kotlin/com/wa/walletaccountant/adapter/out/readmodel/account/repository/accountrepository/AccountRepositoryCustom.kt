package com.wa.walletaccountant.adapter.out.readmodel.account.repository.accountrepository

import com.wa.walletaccountant.domain.account.account.AccountId
import com.wa.walletaccountant.domain.common.Date

interface AccountRepositoryCustom {
    fun updateCurrentMonth(id: AccountId, currentMonth: Date): Boolean
}

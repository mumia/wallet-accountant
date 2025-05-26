package com.wa.walletaccountant.adapter.out.readmodel.account.repository.accountrepository

import com.wa.walletaccountant.application.model.account.AccountModel.ActiveMonth
import com.wa.walletaccountant.domain.account.account.AccountId

interface AccountRepositoryCustom {
    fun updateActiveMonth(id: AccountId, activeMonth: ActiveMonth): Boolean
}

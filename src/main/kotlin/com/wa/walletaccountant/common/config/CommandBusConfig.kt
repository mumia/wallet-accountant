package com.wa.walletaccountant.common.config

import com.wa.walletaccountant.application.interceptor.command.tagcategory.AddNewTagToExistingCategoryCommandInterceptor
import com.wa.walletaccountant.application.interceptor.command.tagcategory.AddNewTagToNewCategoryCommandInterceptor
import org.axonframework.commandhandling.CommandBus
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.context.annotation.Configuration

@Configuration
class CommandBusConfig {
    @Autowired
    fun registerInterceptors(
        commandBus: CommandBus,
        addNewTagToNewCategoryCommandInterceptor: AddNewTagToNewCategoryCommandInterceptor,
        addNewTagToExistingCategoryCommandInterceptor: AddNewTagToExistingCategoryCommandInterceptor,
    ) {
        commandBus.registerDispatchInterceptor(addNewTagToNewCategoryCommandInterceptor)
        commandBus.registerDispatchInterceptor(addNewTagToExistingCategoryCommandInterceptor)
    }
}

package com.wa.walletaccountant.application.interceptor.command.tagcategory

import com.wa.walletaccountant.application.interceptor.service.TagCategoryValidator
import com.wa.walletaccountant.application.interceptor.service.TagValidator
import com.wa.walletaccountant.domain.tagcategory.command.AddNewTagToExistingCategoryCommand
import org.axonframework.commandhandling.CommandMessage
import org.axonframework.messaging.MessageDispatchInterceptor
import org.springframework.beans.factory.annotation.Autowired
import org.springframework.stereotype.Service
import java.util.function.BiFunction

@Service
class AddNewTagToExistingCategoryCommandInterceptor
@Autowired
constructor(
    private val tagCategoryValidator: TagCategoryValidator,
    private val tagValidator: TagValidator,
) : MessageDispatchInterceptor<CommandMessage<*>> {
    override fun handle(messages: List<CommandMessage<*>>): BiFunction<Int, CommandMessage<*>, CommandMessage<*>> {
        return BiFunction<Int, CommandMessage<*>, CommandMessage<*>> { _: Int?, command: CommandMessage<*> ->
            val payload = command.payload as? AddNewTagToExistingCategoryCommand ?: return@BiFunction command

            tagCategoryValidator.validateTagCategoryIdExists(payload.tagCategoryId)
            tagValidator.validateTagCanBeAdded(payload.newTag.tagId, payload.newTag.name)

            return@BiFunction command
        }
    }
}

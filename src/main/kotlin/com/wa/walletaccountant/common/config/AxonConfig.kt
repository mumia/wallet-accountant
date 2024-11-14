package com.wa.walletaccountant.common.config

import org.axonframework.config.Configurer
import org.axonframework.eventhandling.TrackedEventMessage
import org.axonframework.eventhandling.TrackingEventProcessorConfiguration
import org.axonframework.messaging.StreamableMessageSource
import org.springframework.context.annotation.Bean
import org.springframework.context.annotation.Configuration

@Configuration
class AxonConfig {
    @Bean
    fun trackingEventProcessorConfiguration(): TrackingEventProcessorConfiguration =
        TrackingEventProcessorConfiguration
            .forSingleThreadedProcessing()
            .andInitialTrackingToken { obj: StreamableMessageSource<TrackedEventMessage<*>?> -> obj.createHeadToken() }

    @Bean
    fun configurer(configurer: Configurer): Configurer {
        configurer
            .eventProcessing()
            .usingTrackingEventProcessors() // Enable tracking event processors globally

        return configurer
    }
}

package com.wa.walletaccountant.common.config

import org.axonframework.config.Configurer
import org.axonframework.config.ConfigurerModule
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
    fun eventProcessingConfigurerModule(): ConfigurerModule =
        ConfigurerModule { configurer: Configurer ->
            configurer.eventProcessing { eventProcessingConfigurer ->
                // Enable tracking event processors globally
                eventProcessingConfigurer.usingTrackingEventProcessors()
            }
        }
}

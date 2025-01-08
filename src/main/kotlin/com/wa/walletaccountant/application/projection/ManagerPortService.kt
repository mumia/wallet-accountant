package com.wa.walletaccountant.application.projection

import com.wa.walletaccountant.application.port.`in`.ProjectionManagerPort
import org.axonframework.config.EventProcessingConfiguration
import org.axonframework.eventhandling.GlobalSequenceTrackingToken
import org.axonframework.eventhandling.TrackingEventProcessor
import org.axonframework.eventhandling.tokenstore.UnableToClaimTokenException
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.springframework.stereotype.Component
import java.util.Optional


@Component
class ManagerPortService(val eventProcessingConfiguration: EventProcessingConfiguration) : ProjectionManagerPort {
    companion object {
        private val log: Logger = LoggerFactory.getLogger(this::class.java)
    }

    override fun getAllProjectors(): List<String> {
        return eventProcessingConfiguration.eventProcessors().map { it.key }
    }

    override fun restartProjector(projectorName: String) {
        val processors = eventProcessingConfiguration.eventProcessors()

        if (!processors.containsKey(projectorName)) {
            // Key not found!!!!
            return
        }

        val trackingEventProcessor: Optional<TrackingEventProcessor> =
            eventProcessingConfiguration.eventProcessor(
                projectorName,
                TrackingEventProcessor::class.java
            )
        if (trackingEventProcessor.isEmpty()) {
            //processor missing
            return
        }
        val tep: TrackingEventProcessor = trackingEventProcessor.get()
        log.info("Restarting: {} {}", projectorName, processors.get(projectorName))

        tep.shutDown()

        if (!tep.supportsReset()) {
            log.error("Projector does not support reset")

            return
        }
        val position = GlobalSequenceTrackingToken(0)

        try {
            tep.resetTokens(position)
        } catch (exception: UnableToClaimTokenException) {
            // Ignore this exception and let the caller know setting the replay failed.
            log.warn(
                "Unable to claim token for trackingEventProcessor {} on id {}",
                projectorName,
                position.toString(),
                exception
            )

            return
        } finally {
            log.info(
                "Starting replay for trackingEventProcessor {} on id {}",
                projectorName,
                position.toString()
            )

            tep.start()
        }

        log.info("Restarted tracking event processor: {}", projectorName)
    }
}
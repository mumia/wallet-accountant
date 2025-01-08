package com.wa.walletaccountant.adapter.`in`.web

import com.wa.walletaccountant.application.port.`in`.ProjectionManagerPort
import org.springframework.http.HttpStatus.OK
import org.springframework.web.bind.annotation.GetMapping
import org.springframework.web.bind.annotation.PathVariable
import org.springframework.web.bind.annotation.RequestMapping
import org.springframework.web.bind.annotation.ResponseStatus
import org.springframework.web.bind.annotation.RestController

@RestController
@RequestMapping("/api/v1")
class ProjectionManagerController(
    private val projectionManagerPort: ProjectionManagerPort
) {
    @GetMapping(path = ["projectors"], produces = ["application/json"])
    @ResponseStatus(OK)
    fun getAllProjectors(): List<String> = projectionManagerPort.getAllProjectors()

    @GetMapping(path = ["projector/{projectorName}/restart"])
    @ResponseStatus(OK)
    fun readAllAccounts(@PathVariable projectorName: String) {
        projectionManagerPort.restartProjector(projectorName)
    }
}
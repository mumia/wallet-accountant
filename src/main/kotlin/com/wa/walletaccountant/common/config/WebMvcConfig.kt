package com.wa.walletaccountant.common.config

import org.springframework.context.annotation.Configuration
import org.springframework.web.servlet.config.annotation.CorsRegistry
import org.springframework.web.servlet.config.annotation.EnableWebMvc
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer

@Configuration
@EnableWebMvc
class WebMvcConfig : WebMvcConfigurer {
    override fun addCorsMappings(registry: CorsRegistry) {
        registry
            .addMapping("/api/**")
            .allowedOrigins("http://localhost:3000")
            .allowedMethods("POST", "GET", "PUT", "PATCH", "OPTIONS")
            .allowedHeaders("Origin", "Content-Action", "Content-Type")
            .exposedHeaders("Content-Length", "Content-Action")
            .allowCredentials(true)
            .maxAge(43200) // 12h
    }
}
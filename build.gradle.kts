plugins {
    kotlin("jvm") version "1.9.25"
    kotlin("plugin.spring") version "1.9.25"
    id("org.springframework.boot") version "3.3.5"
    id("io.spring.dependency-management") version "1.1.6"
}

group = "com.wa"
version = "0.0.1-SNAPSHOT"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("org.jetbrains.kotlin:kotlin-reflect")

    // Spring
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-validation")
    implementation("org.springframework.boot:spring-boot-starter-data-mongodb")
    implementation("jakarta.validation:jakarta.validation-api:3.1.0")
    implementation("org.hibernate.validator:hibernate-validator:8.0.1.Final")

    // Axon framework
    implementation(platform("org.axonframework:axon-bom:4.10.2"))
    implementation("org.axonframework:axon-spring-boot-starter")
    implementation("org.axonframework:axon-server-connector")
    implementation("org.axonframework.extensions.kotlin:axon-kotlin")

    // Helper
    implementation("org.apache.commons:commons-text:1.12.0")
    implementation("com.google.guava:guava:33.2.1-jre")

    // Logging
    implementation("org.slf4j:slf4j-api")

    // Testing
    testImplementation("org.springframework.boot:spring-boot-starter-test") {
        exclude(module = "mockito-core")
    }
    testImplementation("org.junit.jupiter:junit-jupiter-api")
    testRuntimeOnly("org.junit.jupiter:junit-jupiter-engine")
    testImplementation(kotlin("test"))
    testImplementation("com.ninja-squad:springmockk:4.0.2")
    testImplementation("org.axonframework:axon-test")
    testImplementation("org.axonframework.extensions.kotlin:axon-kotlin-test")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
    implementation("org.testcontainers:testcontainers-bom:1.20.4")
    testImplementation("org.testcontainers:testcontainers")
    testImplementation("org.testcontainers:junit-jupiter")
    testImplementation("org.testcontainers:mongodb")
}

kotlin {
    compilerOptions {
        freeCompilerArgs.addAll("-Xjsr305=strict", "-Xemit-jvm-type-annotations")
    }
}

tasks.withType<Test> {
    useJUnitPlatform()
}

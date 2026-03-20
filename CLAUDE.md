# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Wallet Accountant — a Kotlin-based personal finance/wallet accounting application built on CQRS and Event Sourcing. The project is in early stages.

## Architecture

- **Pattern:** CQRS + Event Sourcing via Axon Framework v5 
  - Use immutable entity pattern
  - Use a generic aggregate creation retry interceptor that handles duplicate ID conflicts via an interface-based approach
- **Command side:** Aggregates handle commands and emit events; events persisted in Axon Server
- **Query side:** Projections consume events and build read models stored in MongoDB
- **Axon Server:** Event store and message router (commands, events, queries)
- **Durable Execution and api server:** Use Restate 1.x (https://www.restate.dev), all api communications and workers run in/through Restate. 
  - Only system api (health check, etc.) should live in the application 

## Language & Tooling

- **Language:** Kotlin
- **Build system:** Gradle
- **Framework:** Axon Framework (v5 latest)
- **Event store:** Axon Server
- **Read model DB:** MongoDB
- **Durable execution:** Restate 1.x (https://www.restate.dev)
- **Build:** `./gradlew build`
- **Test all:** `./gradlew test`
- **Test single class:** `./gradlew test --tests "com.example.MyTestClass"`
- **Test single test:** `./gradlew test --tests "com.example.MyTestClass.myTestMethod"`

## Speckit Workflow

This repo uses speckit for feature specification and planning. Feature artifacts live in `.specify/` and custom Claude commands in `.claude/commands/speckit.*.md`. Use the `/speckit.*` slash commands to drive the spec-plan-task-implement workflow.

## Active Technologies
- Kotlin 2.3.0 (stable `kotlin.uuid.Uuid` with `@OptIn` compiler flag) + Axon Framework 5.0.3, Spring Boot 3.5.3, Jackson
- Axon Server (event store)
- MongoDB

# ADR-0001: Go Fiber Monolith Serving API and Web

Status: Accepted  
Date: 2026-02-28

## Context

The server entrypoint initializes one Fiber app that:

- Registers all API routes under `/api`.
- Serves static/admin assets and WASM artifacts.
- Exposes `/metrics`.
- Runs background jobs in-process.

Deployment and local workflows build and run a single server binary.

## Decision

Use a single Go Fiber monolith as the runtime architecture for API, static/web delivery, and scheduled jobs.

## Consequences

- Single-process deployment and operations remain simple.
- In-process integrations avoid network hops between internal components.
- Independent scaling/isolation of API, web serving, and jobs is limited.

# Changelog

All notable changes to the `flowi-lib` library are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/) and the file is generated from Conventional Commits
by `git cliff` (see `cliff.toml`).

## [Unreleased]

### Features

- **otel:** one trace per run, a span per node and hook, OTLP/HTTP JSON export, optional `traceparent`
- **telemetry:** a closed catalogue of what usage reporting may ever send, with a preview
- **announce:** an Ed25519-signed announcements feed with cache, timeout and rollback protection
- **changelog:** read `CHANGELOG.md` for "What's new"
- **blueprints:** reusable flow templates with typed parameters
- **digest:** webhook styles for Slack, Discord, Teams and email relays
- **http:** HTTP/3 over QUIC
- **serve:** `Workspace.Serve` with the replayer, triggers, schedules and a shared store

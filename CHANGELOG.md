# Changelog

All notable changes to the Flow-I desktop app are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/) and the file is generated from Conventional Commits
by `git cliff` (see `cliff.toml`); the Welcome tab shows the newest release as "What's new".

## [Unreleased]

### Features

- **requests:** browse every mounted collection, edit requests and their versions, and send them with the environment you pick
- **drafts:** try requests with no folder chosen; save a draft to a folder of your own when you are ready
- **flows:** a canvas to build, run and watch flows
- **replayer:** serve saved responses as a fake API and watch the calls arrive (Workspace › Replayer)
- **telemetry:** an explanation, a preview and the choice, in Help › Telemetry; off by default
- **announcements:** signed project notices on the Welcome tab when a feed is configured
- **what's new:** this list, read from the changelog built into the app
- **tracing:** export OpenTelemetry traces when `otel.endpoint` is set in Settings

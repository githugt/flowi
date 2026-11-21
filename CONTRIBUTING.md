# Contributing to Flowi

Thank you for helping. This repository has three parts that build on each other: `lib` (the core library), `cli`
(the command line) and `app` (the desktop app). Each has an `AGENTS.md` that lists its rules. Read the one for the
part you change. The rules apply to people too.

## Before you start

- Open an issue for a large change, so the design can be agreed before the work is done.
- Do not report security problems in a public issue. See `SECURITY.md`.

## Making a change

1. Work in the part that owns the behaviour. Logic goes in `lib`; `cli` and `app` stay thin.
2. Add a test for every behaviour change. Say in a comment why the old behaviour was wrong (`// Before: …`).
3. Run the checks for the part you changed: `just check` in `lib`, `cli` or `app`. They tidy, lint and run the tests
   with the race detector. They take a few minutes.
4. Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, …). The changelog is
   generated from them.
5. Open a pull request. Say what changed and how you checked it.

## What not to do

- Do not put secrets in files, logs, tests or fixtures. Use obviously fake values.
- Do not add a dependency without a reason. Every dependency must build with `CGO_ENABLED=0` in `lib`.
- Do not hide an error. If you ignore one, write why.

## Licences

`lib` is Apache-2.0. `cli` and `app` are AGPL-3.0. A file carries the licence of the part it is in. By
sending a change you agree that it is licensed the same way.

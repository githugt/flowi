# Security Policy

This repository holds three parts: `lib` (Apache-2.0), `cli` and `app` (AGPL-3.0). The same policy applies to all of them.

## Supported versions

Security fixes are made on the latest minor release only.

## Reporting a vulnerability

Please **do not** open a public issue for security problems.

Report them privately through GitHub's
[private vulnerability reporting](https://github.com/githugt/flowi/security/advisories/new)
for this repository. Include the affected version, steps to reproduce, and the impact you observed.
You will get an acknowledgement within 7 days.

Flowi handles secrets (environment values marked secret, vault keys). Reports about secrets leaking into logs,
reports, saved responses or committed files are especially welcome.

Each part also has its own file (`lib/SECURITY.md`, `cli/SECURITY.md`, `app/SECURITY.md`) with details for that part.

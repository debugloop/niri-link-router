# Security policy

This is an unreleased companion application.
Report vulnerabilities through [GitHub private vulnerability reporting](https://github.com/debugloop/niri-link-router/security/advisories/new).
The repository enables this private channel for security reports.
Do not post browser profiles, private URLs, native-message payloads, or AMO credentials in public issues.

Include the affected version, prerequisites, and a redacted reproducer in your private report.
The native bridge trusts the current Unix account and the Firefox native-application allowlist.
See [the threat model](docs/security.md) for socket ownership, validation, and acknowledgement limits.

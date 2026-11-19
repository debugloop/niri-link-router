# Changelog

## 0.2.2

- Route external links to Firefox on the launching window’s workspace when the source can be identified.
- Use the configured fallback workspace when the source is unknown or has no Firefox window.
- Select the new tab without changing window or workspace focus during routed delivery.

## 0.2.1

- Document permanent installation from Firefox Add-ons and optional Nix provisioning.
- Publish through the GitHub Actions submission and signed-artifact collection workflows.
- Keep extension and native-host behavior unchanged from 0.2.0.

## 0.2.0

- Use niri Link Router as the public add-on name.
- Replace protocol-handler navigation with a Firefox-owned native bridge.
- Remove request pages, protocol handlers, pairing, native keys, and legacy transport flags.
- Register the native host through the Nix modules and manual installer.
- Acknowledge tab creation and avoid retries after uncertain submission.
- Apply native deadlines to writes and acknowledgements, including when Firefox stops reading.
- Keep current-workspace routing, configurable fallback workspaces, and ordinary Firefox fallback.
- Preserve profile-wide labels, multiple profiles, and URL-free receipts.

The Mozilla-signed add-on is available on [Firefox Add-ons](https://addons.mozilla.org/en-US/firefox/addon/niri-link-router/).

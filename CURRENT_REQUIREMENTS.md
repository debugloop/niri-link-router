# Current requirements

The user's direct clarifications supersede the original cloud handoff.

## Routing

1. For link opens, prefer Firefox on the launching window’s Niri workspace when known. The `focus` command uses the focused workspace.
2. Otherwise use Firefox on the configured fallback workspace, `web` by default.
3. If routing is unavailable before submission, pass the original URL to Firefox normally.
4. For routed delivery, select the new tab and preserve the current window and workspace focus.
5. Support arbitrary existing profiles and overlapping profile-local Firefox window IDs.
6. Never blindly retry after uncertain submission.

Workspace proximity does not affect routing.
The fallback name belongs in runtime JSON or the Nix configuration, not in edited Go source.

## Native bridge

The user approved a separate native process owned by Firefox, one per participating profile.
There is no independent service, polling loop, or persistent routing cache.
Firefox starts the host and its native port controls the host's lifetime.
The host resolves its Firefox ancestor and locked profile, then listens on a private per-profile Unix socket.
Native acknowledgements distinguish opened, rejected-before-creation, and uncertain delivery.

The new implementation replaces the old protocol transport completely.
There are no request pages, protocol handlers, pairing flows, native keys, or legacy transport modes.
The extension retains profile-wide labels and URL-free receipts.
Both executables use Go's standard library. The extension uses plain JavaScript without a build step.

## Deployment and tests

The native-bridge replacement is now staged in the main checkout at `~/code/niri-firefox-router`.
The temporary development worktree is removed.
The NixOS flake uses the local path input so staged changes do not need a new Git commit.
Activation and the temporary add-on reload still need a coordinated migration.
The user approved automated and disposable-profile desktop tests after the initial implementation pause.
Do not switch unrelated pending NixOS configuration changes as part of the transport migration.
See [native validation](docs/native-validation.md) for evidence and remaining limits.

The old local prototype desktop entry was removed after it shadowed the Nix package and produced incompatible signatures.
Keep imperative desktop overrides out of the final Nix installation.
No signed AMO release exists yet.

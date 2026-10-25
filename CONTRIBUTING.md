# Contributing

The repository contains two Go executables and a plain JavaScript extension.
Keep the implementation free of external Go dependencies and JavaScript build steps.
Do not add an independent daemon, polling loop, profile map, or persistent routing cache.
A Firefox-owned native host per participating profile is intentional.

## Automated checks

```sh
nix develop
nix flake check
```

Individual checks:

```sh
go test -race ./...
go vet ./...
golangci-lint run ./...
node --test tests/extension.test.cjs
python3 tests/cli_smoke.py
python3 -m unittest discover -s tests -p 'test_*.py'
```

Use `gofmt` and `nix fmt` for formatting.
Nix checks also cover the module interfaces, native-host registration, strict extension lint, and release workflows.
CI runs on Linux x86-64 and ARM64. Local evaluation of ARM64 derivations is not native execution.

## Transport invariants

- Identify profiles through Firefox processes and locks, not profile-local window IDs alone.
- Start the native host only as a Firefox descendant.
- Keep runtime directories private and sockets restricted to the current UID.
- Bound messages, request deadlines, reservations, and receipts.
- Distinguish rejection before tab creation from uncertain delivery.
- Never replay submitted requests after a disconnect or timeout.
- Do not put URLs, payloads, or browser-profile data in normal logs or receipts.

Tests must cross the real native-message and Unix-socket interfaces, not an alternate delivery implementation.
The compiled-CLI smoke test uses real sockets without a running browser.
The bridge tests cover process-independent framing, profile identity, acknowledgements, and lifecycle cleanup.

## Desktop tests

Obtain permission before changing focus or using someone's active browser.
Prefer disposable profiles and an isolated runtime directory.
The browser helper registers native manifests only in its temporary home/config directories.
It uses an absolute Wayland display path so runtime isolation does not disconnect it from the compositor.

```sh
python3 tools/start_browser.py --state-dir /tmp/router-browser \
  --native-host ./dist/niri-firefox-router-native-host
```

`tools/native_matrix.py` intentionally changes focus and requires a JSON plan with compositor IDs:

```json
[
  {"name":"local","source":10,"target":11,"recent":12},
  {"name":"fallback","source":20,"target":21,"recent":11}
]
```

```sh
python3 tools/native_matrix.py --router ./dist/niri-firefox-router \
  --runtime-dir /tmp/router-browser/runtime --fallback-workspace web \
  --plan /tmp/router-plan.json --output /tmp/router-result.json
```

A compositor-spawn test must explicitly pass an isolated runtime directory into the spawned command.
A desktop-handler test needs its own XDG desktop entry and MIME configuration.
Never replace the user's actual URL handler merely to run a disposable test.

Check actual HTTP arrivals, selected destination tabs, and focused windows separately.
Count tab-open events and temporary navigations to detect hidden interstitials.
Afterward, stop disposable browsers, check that their native hosts exit, and restore the original workspace.
See [the native record](docs/native-validation.md).

## Releases

Choose a permanent extension ID before publication and keep the native allowlist consistent with it.
Preserve signed XPIs unchanged and keep native and extension versions coordinated.
See [Firefox installation](docs/firefox-installation.md).
Never weaken Firefox Release signature verification.

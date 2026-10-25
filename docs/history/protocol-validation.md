> Historical record for the removed 0.1 protocol transport. This is not current setup or native-bridge evidence.

# Native validation record

Date: 2026-10-25. Desktop: niri 26.04, Firefox Release 158.0, Linux x86-64.
The actual existing test profile's `about:support` reported `Window Protocol: wayland` and `Desktop Environment: niri`.
The cropped [Wayland evidence](native-wayland.png) excludes unrelated desktop content.

The accepted behavior selects the new tab and focuses Firefox.
Earlier cloud instructions about proximity and focus preservation were superseded.
A focus change is not itself a failure.

## Routing tests

[Structured evidence](native-evidence.json) records 34 native cases and 41 successful URL deliveries.
It retains bounded niri event traces without window titles, URLs, pairing keys, or signatures.
The historical routing matrix predates profile-specific key derivation.
Some early cases also preceded the explicit extension-side focus update.
The newer pairing and profile-isolation tests below exercise the final authentication design.

| Case | Evidence |
| --- | --- |
| Existing `personal` profile | Correct local window, selected tab, intended focus |
| Square, triangle, hexagon | Each workspace's own Firefox received its links |
| Bare protocol and `--new-tab` | Both transports delivered once |
| Different most-recent Firefox | The policy still chose the intended local destination |
| No local Firefox | A real workspace switch reached Firefox on `web` |
| Different profile on `web` | Correct profile and window received the URL |
| Three simultaneous profiles | All three could use Firefox window ID `1` without ambiguity |
| Eight concurrent opens | Each URL appeared once in the expected test profile |
| niri spawn | Correct delivery through the compositor launch path |
| Kitty `open_url` action | Local and cross-profile fallback both passed |
| Normal `xdg-open` | Current-workspace and cross-profile fallback passed |
| GLib `gio open` | Both paths passed |
| Existing Kitty mouse click | One request, selected destination tab, correct focused window |
| Existing Kitty mouse click without local Firefox | One request on `web`, with the expected workspace switch |

The tests used bounded local pages with unique titles.
HTTP request counts established delivery separately from CLI status.
The native title and focused-window records identified the selected destination.
Observers subscribed before invocation and stopped afterward.
Activation tokens were not stripped.

The original `web` profile was not used for the intentional multi-profile tests.
Disposable second and third profiles supplied those cases and were then removed.
The existing test profile now has an additional window on `web`.
Blank source terminals were returned to their original workspaces.

An early desktop-entry trial fell back to system Firefox because generic `xdg-utils` mishandled a quoted executable.
Its newly launched test window was closed.
The installer now emits a safe unquoted executable path, with real `xdg-open` argument tests to catch this regression.
This unsuccessful setup trial is not counted as a routing pass.
Some virtual-input trials also missed their intended link and produced no HTTP request.
Only trials with confirmed delivery count toward the record.

## Automatic pairing and profile labels

A separate disposable profile loaded the extension without a key.
The native test checked these steps:

1. The settings page reported an unpaired profile.
2. A profile label changed the native window prefix.
3. The Go `pair` command opened an approval page and waited.
4. Approval saved the key and produced a native acknowledgement.
5. The settings page then reported a paired profile and retained its label.
6. A Go-signed URL opened successfully with the automatically paired key.

The [pairing record](native-pairing.json) contains outcomes, not key or token material.
It also checked master-key reuse and the profile-specific manual fallback.
This exercised Firefox's real Web Crypto, CORS, extension storage, and loopback requests.
It did not require broad host permissions or disabling signature checks.
User activity can disturb desktop focus, so pairing storage/acknowledgement is not inferred from a final focus snapshot.

## Cross-profile authentication

Two disposable profiles paired against the same native master key and both used Firefox window ID `1`.
A captured Go-signed request from the first profile was rejected in the second profile, with its handler cleaned up and no HTTP delivery.
The intended profile then delivered it once and rejected a replay.
The [profile-isolation record](native-profile-isolation.json) stores these outcomes without the captured URI.

The mouse tests used an injected Wayland pointer on the real desktop, not mocked Kitty callbacks.

## Automated tests are separate

The Go tests include real Unix-socket fake compositor peers, profile locks, key permissions, replay wire encoding, deadlines, and a bounded pairing server.
JavaScript tests cover authentication, handler identity, cleanup, concurrent nonces, storage failures, explicit approval, acknowledgement order, and label updates.
Python tests cover the compiled CLI and reversible personal installation.
Go-signed requests also pass the actual JavaScript parser under Node.js.

These tests do not replace native browser evidence.
Nix checks add strict extension lint, archive checks, Go lint, and module-composition checks.
The real Home Manager and NixOS options were also evaluated without applying a configuration.

## Still untested

- Signed persistent installation, full restart, and restored-window timing
- More than one simultaneously active output
- Physical links from applications other than Kitty
- Native ARM64 desktop behavior
- AMO signing and GitHub-hosted workflow execution

No signed release or AMO submission exists.
ARM64 flake derivations were evaluated locally, not executed on an ARM64 machine.

# Native bridge validation

Date: 2026-10-25. Linux x86-64, niri 26.04, Firefox Release 158.0.
The user approved desktop tests after the implementation pause.
The tests used disposable Firefox profiles, isolated native registrations, and a private runtime directory.
The current user profiles and URL-handler configuration were not upgraded.

## Delivery

[Structured evidence](native-bridge-evidence.json) records six successful sequential cases and eight concurrent deliveries.
The cases covered:

1. A local window in the first profile, with another window previously most recent.
2. A fallback workspace and a second window in the same profile.
3. Another profile whose Firefox window ID was also `1`.
4. Fallback after a different profile was most recent.
5. Compositor-spawned invocation.
6. An isolated XDG desktop/MIME handler.

Every successful case received its HTTP request once and selected the expected focused window.
All eight concurrent opens received native acknowledgements and exactly one HTTP request each.
The six traced cases produced six tab-open events and six retained destination tabs.
There were no extension-page or protocol-handler navigations during those links.
The concurrent cases had HTTP and acknowledgement evidence but no final retrieved tab-event trace.

The first compositor-spawn trial omitted the isolated runtime variable from the spawned environment.
It therefore used ordinary Firefox fallback rather than the disposable bridge.
That setup failure is excluded from the routing pass count.
The corrected test explicitly passed the runtime environment and succeeded.
One harmless diagnostic page reached the existing browser during that failed setup trial.

## Lifetime and resources

Each Firefox profile started one native host, including the profile with two browser windows.
The host processes were direct children of their Firefox processes in this test.
A process snapshot measured 4988 KiB and 4544 KiB RSS with 0.0% displayed CPU.
These values are observations, not a resource guarantee or benchmark.

After both disposable browsers closed, neither native host remained and both sockets were gone.
Native connection/reconnect logic also has automated coverage.
A live extension reload and the settings reconnect button still need a native trial.

## Automated checks

The Go tests cover native framing, profile ancestry and locks, private sockets, acknowledgements, failure classification, and cleanup.
A 2026-11-14 regression test covers unread native output, request deadlines, cancellation, and profile-lock recovery with operating-system pipes.
Additional tests cover cancellation during blocked startup reads and writes.
These tests do not use a live browser.
The compiled CLI test uses real Unix sockets and passes its request through the actual JavaScript validator.
JavaScript tests cover direct tab creation, duplicate requests, uncertain failures, labels, receipts, connection startup, and bounded reconnects.
Python tests cover native-host installation, rollback, default-handler argument preservation, and signed-artifact collection.

These checks do not establish behavior on an untested browser package or architecture.
The earlier [protocol record](history/protocol-validation.md) describes a removed implementation and is not native-bridge evidence.

## Remaining native work

- Signed persistent installation, full restart, and restored windows
- Live extension reload and explicit native reconnection
- More than one active output
- Physical clicks from application UIs with the replacement installed
- Native ARM64 execution
- Runtime Wayland confirmation in this test profile, rather than the earlier transport's evidence
- AMO submission and GitHub-hosted release execution

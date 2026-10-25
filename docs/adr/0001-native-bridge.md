# Replace protocol navigation with a Firefox-owned native bridge

Status: accepted by the user, 2026-10-25.

## Problem

A WebExtension protocol handler requires Firefox to navigate to an extension page before the extension receives the request.
That page flashes on ordinary link clicks.
Blank styling cannot remove the temporary tab itself, and Firefox 158 Release has no supported background-URL switch.

## Alternatives

1. Keep the protocol handler and hide its contents. This retains the transient tab and old pairing transport.
2. Publish a connection token in window titles. This adds visible endpoint metadata and changes the title format.
3. Use a Firefox-owned native host and identify its profile through its process ancestry and Firefox's profile lock.

The user approved option 3 and explicitly requested removal, not preservation, of the old transport.
The earlier prohibition on persistent native helpers now permits a native child process for each participating Firefox profile.
Independent services, periodic polling, and routing caches remain out of scope.

## Interface

The CLI still selects a niri window and resolves its Firefox PID to a canonical locked profile.
A socket filename derived from that path identifies the live host; it does not store a routing decision.
The host independently resolves its Firefox ancestor to the same profile.
Private runtime directories, socket permissions, Unix credentials, and the native manifest define the local trust boundary.
The current Unix account is trusted.

The native host forwards one bounded request at a time over Firefox's standard native-messaging framing.
Replies acknowledge tab creation, definite rejection before creation, or uncertain delivery.
The CLI falls back to ordinary Firefox only before submission or after a definite rejection.
It does not replay uncertain requests.

## Consequences

There is no handler tab, protocol registration, approval page, pairing key, or legacy transport mode.
The native host runs only while Firefox owns its connection.
The extension keeps a persistent background connection and performs bounded reconnect attempts after failures.
Both executables remain standard-library Go programs.

Installation now includes a native-application manifest and the extension's `nativeMessaging` permission.
Native and extension updates must be coordinated.
The Firefox-process/profile identity contract needs native tests for unusual browser packaging.

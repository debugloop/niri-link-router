# Native bridge security

## Trust boundary

The extension connects only to the registered `niri_firefox_router` native application.
The native manifest permits only the router extension ID.
The extension exposes no protocol handler, content script, or web-accessible resource.
Websites cannot submit native messages or connect to the Unix socket through browser networking.

The native host must descend from a Firefox process owned by the current Unix user.
It walks a bounded parent chain and identifies the browser through its command line and locked profile.
A direct shell invocation without that Firefox ancestor cannot start a listening host.
Profiles with `--no-remote` are supported because delivery does not use Firefox's remote command line.

This design trusts other processes with control of the same Unix account.
It does not protect against malware that can replace executables, edit native manifests, or read browser profiles.
There are no pairing secrets or shared master keys in this transport.

## Profile identity and socket ownership

The CLI joins a niri window's PID to its locked Firefox profile.
The host independently resolves its Firefox ancestor's locked profile.
Both use the canonical profile path to derive a socket filename through SHA256, truncated to 128 bits.
The filename is an endpoint identifier, not a credential or persistent routing map.
Firefox window IDs remain local to their owning profiles.

Sockets and lock files live under `$XDG_RUNTIME_DIR/niri-firefox-router`.
Both the runtime directory and child directory must be private, user-owned directories rather than symlinks.
Socket permissions are 0600. Both sides check Unix peer credentials for the current UID.
The host sends its Firefox PID before the client submits a URL; the CLI checks it against the selected process.
The host holds an exclusive advisory lock and removes a stale socket only after it acquires that lock.
A lock file can remain until logout, but it contains no profile path, request, or key.

No TCP listener, HTTP bootstrap server, or periodic polling runs in the installed router.
Firefox starts one native host per participating profile and owns its standard-input/output pipes.
The host removes its socket when Firefox disconnects.
A bounded reconnect sequence handles transient startup or reload races. It does not retry indefinitely.

## Framing and validation

Native messages use a four-byte little-endian length followed by one JSON object.
Frames are limited to 64 KiB on both the Unix socket and Firefox connection.
Go rejects unknown fields, incomplete frames, oversized frames, and trailing JSON values.
Each request contains a protocol version, command, random 128-bit request ID, window ID, URL, tab-selection flag, and deadline.
Requests expire within at most 30 seconds.

Both Go and JavaScript accept only absolute HTTP/HTTPS URLs with a host and no credentials.
They reject controls, whitespace, malformed escapes, backslashes, and URLs over 8192 UTF-8 bytes.
Firefox performs its own URL parsing afterward.
Normal fallback uses the same validated original URL as one executable argument, never a shell command or `xdg-open` recursion.

The extension serializes requests and reserves IDs in session storage before tab creation.
The reservation set is bounded and expires with request deadlines.
A repeated ID produces an uncertain result rather than an instruction to repeat delivery.
Private or non-normal destination windows are rejected before tab creation.

## Acknowledgement and uncertain delivery

`opened` means `tabs.create` completed successfully.
A later focus or receipt error cannot change it into a rejected delivery.
It does not mean the target page finished its network load.

`rejected` means the extension made no tab-creation attempt for this request.
The CLI can then use ordinary Firefox handling.
`uncertain` means a tab-creation attempt failed ambiguously or a duplicate request ID appeared.
Missing, mismatched, or late acknowledgements after submission are also uncertain.

The CLI uses ordinary Firefox handling if routing or connection setup fails before submission.
Once submission starts, it never retries automatically unless an explicit rejection establishes that tab creation did not start.
One deadline covers the native write and its acknowledgement.
The host uses duplicate, nonblocking pipe descriptors so Go can interrupt blocked reads and writes.
Cancellation closes both native pipes, including during startup.
A submission timeout closes that native connection, preventing a late reply from matching a later request.
A failed host releases its socket and profile lock.
A new connection never replays a previous request.

## Diagnostic data

Normal logs do not include request URLs or native payloads.
Receipts contain only time and one of `opened`, `rejected`, or `uncertain`, with a maximum of 32 entries.
Unexpected storage fields are discarded when receipts are rewritten.
A receipt failure cannot cause another tab creation.
Dry-run output contains window-selection metadata without the URL.
Native-host standard output contains only framed messages; errors go to standard error.

## Labels and privacy

A profile-wide label is a display setting, not an authentication mechanism.
The router retains its own `[nfr:<id>]` prefix.
Labels reject control characters and nested router prefixes.
A storage error leaves an existing title prefix alone.
For sensitive screen sharing, use compositor rules that fail closed before a window receives its label.

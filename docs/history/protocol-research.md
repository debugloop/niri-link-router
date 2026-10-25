> Historical record for the removed 0.1 protocol transport. This is not current setup or native-bridge evidence.

# Architecture research

Research date: 2026-10-25.
The cloud handoff initially requested workspace proximity and focus preservation.
The user replaced those requirements with same-workspace/web routing and an explicit focus change.
The primary-source findings below remain useful, but the old focus gate is not the current acceptance criterion.

## Primary sources

1. [open-url-in-container source](https://github.com/honsiorovskyi/open-url-in-container), commit `4fa376570305de73e00b3ffc99adbbf241bed099`.
   The inspected manifest identifies version `2.0.0alpha10`, not the stable release.
   It registers `ext+container` with `/opener.html#%s`.
   Its opener loads a real handler tab, creates a destination tab, and removes the handler tab.
   Its tab helper does not preserve compositor focus explicitly.
   Newer security code uses Web Crypto HMAC-SHA256.
   The checked-in shell launcher still hashes `name + secret` with SHA256, which does not match that newer authentication code.
   We did not copy either implementation.
   Upstream code uses MPL-2.0.
2. [Issue 121](https://github.com/honsiorovskyi/open-url-in-container/issues/121) reports repeated Firefox protocol prompts.
   Commenters report `network.protocol-handler.external.ext+container = true` as a workaround.
   That browser prompt differs from the extension's unsigned-request confirmation.
   The native tests here used stock `ext+nfr` behavior and did not need the analogous preference.
   They do not prove behavior after persistent installation or browser restart.
   Do not disable global protocol warnings.
3. [MDN protocol_handlers](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/manifest.json/protocol_handlers) documents navigation to a URL template.
   `%s` receives the escaped complete handled URL.
   No supported tab-free callback appears in this API.
   An event page can process a message, but it does not eliminate the initial handler navigation.
4. [MDN windows.update](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/windows/update) documents `titlePreface`.
   It warns that untitled windows, such as `about:blank`, can behave differently across operating systems.
5. [MDN tabs.create](https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/API/tabs/create) distinguishes an active tab from a focused window.
   `active: true` selects the tab and does not, by contract, focus its window.
   That contract does not cover activation during the preceding external invocation.
6. [niri IPC documentation](https://github.com/niri-wm/niri/blob/main/docs/wiki/IPC.md), [types](https://github.com/niri-wm/niri/blob/main/niri-ipc/src/lib.rs), and [socket helper](https://github.com/niri-wm/niri/blob/main/niri-ipc/src/socket.rs) define newline-delimited JSON over `$NIRI_SOCKET`.
   The installed 26.04 instance accepted `Workspaces`, `Windows`, `EventStream`, and the documented focus action.
   Workspace `id` is stable while `idx` changes with current per-output order.
   `is_active` identifies each output's visible workspace.
   Optional monotonic `focus_timestamp` supplies recent-focus history.
   The event stream supplies initial state followed by changes.

## Existing extensions

1. [Tab'o'mat URL to Window Router](https://addons.mozilla.org/firefox/addon/tab-o-mat-url-to-window-router/), inspected XPI version 0.35.0.
   It observes new tabs and moves them according to domain or project rules.
   It offers optional destination focus.
   Its policy does not read niri workspace order or the external source workspace.
2. [Open Link in Other Window](https://github.com/kyuucr/open-link-in-other-window), inspected AMO artifact `5resigned1`.
   It creates tabs in a manually selected window from an in-browser context menu.
   Its optional `windows.update({focused: true})` differs from the router's ordinary-open requirement.
3. [Other Window](https://addons.mozilla.org/firefox/addon/otherwindow/), inspected XPI version 5.1.0.
   It supplies menu-button and context-menu actions for tab/link movement.
   Users can choose whether an action focuses the destination and can name windows.
   It does not implement external workspace-proximity routing.

These implementations show useful tab APIs, but none meets the complete external routing requirement without additional integration.
An ordinary Firefox URL invocation does not convey the selected Firefox window ID.
Therefore, this prototype tests an authenticated protocol request rather than a domain-rule or manual-selection extension.

## Firefox command-line support

The installed browser reports `Mozilla Firefox 158.0`.
Its actual `--help` does not list `--url-background`.
The installed `BrowserContentHandler.sys.mjs`, extracted from `browser/omni.ja`, also lacks that switch.
The inspected upstream [release branch](https://github.com/mozilla-firefox/firefox/blob/release/browser/components/BrowserContentHandler.sys.mjs) lacks it.
The inspected [main branch](https://github.com/mozilla-firefox/firefox/blob/main/browser/components/BrowserContentHandler.sys.mjs) contains it.
These branch references can change.

The CLI exposes `url-background` only for future experiments on a browser that supports it.
We did not test it on this Release.
An ignored switch and an undelivered URL cannot establish focus preservation.

## Historical alternatives for focus-preserving delivery

1. Test a version-supported background URL invocation, with separate delivery evidence.
2. Examine narrowly scoped child activation-token handling without changing the parent environment.
3. Examine the installed niri version's activation policy and document its tradeoffs.
4. Request upstream Firefox support for background, window-directed external delivery.
5. Reconsider native messaging only if its trigger and lifecycle need no persistent helper.

A CLI cannot initiate arbitrary native messages into an extension.
A long-lived native host would violate this project's central requirement.
These alternatives are not necessary for the clarified behavior, which intentionally focuses Firefox.

# AMO submission worksheet

Use this worksheet for the first listed submission to addons.mozilla.org (AMO).
The text below describes version 0.2.3.
The permanent extension ID is `niri-firefox-router@danieln.de`.

## Owner decisions before submission

- [x] Choose the permanent extension ID: `niri-firefox-router@danieln.de`.
- [x] Choose the public repository: <https://github.com/debugloop/niri-link-router>.
- [ ] Publish the source revision for this version and record it in the reviewer notes.
- [ ] Choose the maintainer display name and public support channel.
- [x] Use GitHub private vulnerability reporting, as documented in `SECURITY.md`.
- [ ] Publish the companion application source and installation instructions.
- [ ] Replace the reviewer placeholders in [the review notes](amo-reviewer-notes.md).

An email-style ID usually uses a domain or stable author namespace after `@`.
It does not need a mailbox or a live website.
Mozilla checks the complete ID for uniqueness.
A domain under the maintainer's control helps avoid naming conflicts.

The permanent ID uses the maintainer's `danieln.de` domain.
The ID identifies the extension, its storage, and its native-host authorization.
Keep it unchanged after the first submission.
It is separate from the AMO listing slug and the maintainer's email address.

`browser_specific_settings.gecko.id` in `extension/manifest.json` defines the permanent ID.
The Nix package, manual installer, test helper, and artifact collector read this value from the manifest.
Rebuild the native package when you replace an installation with the old development ID.
Update the installed native-host registration before testing the permanent extension ID.
Do not change generated files under `/nix/store`.

## Listing fields

### Name

niri Link Router

### Suggested slug

`niri-link-router`

AMO checks whether this slug is available.
Do not use Mozilla or Firefox trademarks in the add-on display name.
The description can explain that the extension works with Firefox.
The extension ID and companion command remain unchanged.

### Summary

Open external links in Firefox on your current niri workspace, with a configurable fallback.

### Description

Open external HTTP and HTTPS links in Firefox on your current niri workspace.
If that workspace has no Firefox window, use a configured fallback workspace, named `web` by default.
The new tab is selected and the destination Firefox window takes focus.

This extension requires the separate niri-firefox-router companion application on Linux.
Install the companion application and register its native host before use.
The extension does not route links by itself.
Installation instructions: https://github.com/debugloop/niri-link-router

Features:

- Use an existing Firefox window on the current workspace or the fallback workspace.
- Keep multiple Firefox profiles separate.
- Open the requested page directly, without an intermediate extension tab.
- Set an optional profile-wide window label.
- Check native-bridge status and recent delivery results in the extension settings.

Requirements and limits:

- Linux with the niri compositor and Firefox 142 or later on native Wayland.
- The companion application and extension in each participating profile.
- Private browsing windows are not supported.
- Sandboxed native-messaging portals, such as Flatpak Firefox, are not supported.
- Disable Window Titler because it can overwrite the required window-title prefix.
- Each participating window receives a visible `[nfr:<id>]` title prefix.
- When routing is unavailable before submission, the companion command uses ordinary Firefox handling.
  Firefox then chooses the destination window, which can be on another workspace.

The extension has no analytics, advertising, or remote service.
It receives requested URLs from the local companion application and asks Firefox to open them.
Firefox then contacts the destination website normally.
Delivery records contain only a status and timestamp, never the URL.

### Other fields

| Field | Proposed value |
| --- | --- |
| Distribution | On this site, listed |
| License | MIT |
| Platform | Linux only, desktop Firefox |
| Category | Tabs, if available in the current form |
| Suggested tags | niri, workspace, tabs, linux |
| Experimental | Yes for the first release |
| Homepage | <https://github.com/debugloop/niri-link-router> |
| Support | Proposed: <https://github.com/debugloop/niri-link-router/issues> |
| Privacy policy | Paste [the privacy statement](privacy.md) |
| Reviewer notes | Paste [the review notes](amo-reviewer-notes.md) after replacing placeholders |
| Separate extension source | No generated JavaScript. The XPI contains the readable source |

Do not select Android, Windows, or macOS support.
If AMO requests companion source, supply the matching source archive and its revision.
No account or paid service is required to use the extension.

## Build and inspect

Run these commands after the source is final:

```sh
nix flake check --print-build-logs
nix build .#firefox-extension-unsigned --out-link result-amo-unsigned
unzip -p result-amo-unsigned manifest.json
unzip -l result-amo-unsigned
sha256sum result-amo-unsigned
```

Upload the file that `result-amo-unsigned` points to.
Do not upload the companion source archive as the extension package.
The unsigned XPI cannot serve as a permanent installation in stock Firefox Release.

The submission form can change independently of this worksheet.
Read each current question before you accept its proposed answer.

## Submit

1. Sign in to the [AMO Developer Hub](https://addons.mozilla.org/developers/).
2. Complete Mozilla's account, agreement, and authentication requirements.
3. Select **Submit a New Add-on**.
4. Select **On this site**.
5. Upload the unsigned XPI.
6. Read the validation results.
7. Select Linux compatibility only.
8. Supply the listing fields, privacy statement, and reviewer notes.
9. Submit the version.
10. Record the assigned listing URL and submission status.

Signing or publication can precede a later human review.
Mozilla can request changes or reject a version after publication.
Do not promise a fixed signing or review time.
If an upload times out, check AMO before you retry the same version.

## After signing

1. Download Mozilla's signed XPI without modifying it.
2. Install it through AMO or **Install Add-on From File** in `about:addons`.
3. Check that the native bridge reports **Connected**.
4. Test a complete Firefox restart and restored windows.
5. Test links from the current workspace and a workspace without Firefox.
6. Complete the remaining tests in [the release checklist](release-plan.md).

For Nix provisioning, use the signed XPI with `lib.mkSignedExtension`.
See [Nix integration](nix.md).

The first submission does not need AMO API credentials.
The existing release workflows use GitHub Actions in `debugloop/niri-link-router`.
Configure the submission workflow only after the listing exists.
Manual submission and collection also remain available.
Do not tag this already-submitted version merely to trigger another submission.
Use a new manifest version for each later submission.
See [release automation](firefox-installation.md).

## References

- [Extension IDs](https://extensionworkshop.com/documentation/develop/extensions-and-the-add-on-id/)
- [Submission steps](https://extensionworkshop.com/documentation/publish/submitting-an-add-on/)
- [Add-on policies](https://extensionworkshop.com/documentation/publish/add-on-policies/)

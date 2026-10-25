# Firefox installation and release automation

## One signed add-on for all users

Use a regular listed AMO add-on for both Nix and non-Nix installations.
Firefox Release requires Mozilla signing in either case.
Nix users can provision the same signed XPI through Home Manager or NixOS policies.

A self-signed certificate is not a supported substitute in stock Firefox Release.
Importing a CA certificate does not change Firefox's add-on signing authority.
Enterprise force-install policies control installation, not whether an unsigned or self-signed XPI is trusted.

Some development/ESR builds allow signature checks to be disabled.
That is a different browser configuration and security model, not selective trust for a Nix-provisioned bundle.
A custom Firefox build could also change this behavior, but maintaining a browser fork is not the installation method here.
Do not disable signature verification in the modules.

## Initial AMO setup

Use [the submission worksheet](amo-submission.md) for listing text, owner decisions, and upload checks.
It links to the privacy statement and reviewer test instructions.

The owner must do these steps once:

1. Create an account at the [AMO Developer Hub][amo].
2. Complete Mozilla's current account, agreement, and authentication requirements.
3. Check that the extension and native host use the permanent ID, `niri-firefox-router@danieln.de`.
4. Run `nix flake check` and build `nix build .#firefox-extension-unsigned`.
5. Create the listed add-on through the Developer Hub and upload the unsigned XPI.
6. Supply the license, listing metadata, and required privacy information.
7. Explain the native host, Unix socket ownership, nativeMessaging permission, and profile labels in the review notes.
8. Check the submission status and answer any review requests.
9. Create AMO API credentials for subsequent automated submissions.

The permanent ID is `niri-firefox-router@danieln.de`.
Keep it unchanged after the first submission.
Existing development installations need an updated native-host allowlist before they can use this ID.
Signing or publication can precede a later human review.
Mozilla can request changes after publication, and review times vary.

The source is plain JavaScript without a build step.
The manifest requires Firefox 142 or later and declares no remote data collection.
No signed artifact, public listing, or AMO credentials exist in this checkout.

## Automation and repository hosting

The project repository is [debugloop/niri-link-router on GitHub](https://github.com/debugloop/niri-link-router).
The existing workflows use GitHub Actions.
Publish the source and configure the credentials and permissions below before you use automatic submission.
The manual submission and collection commands also remain available.

## Automatic submission on GitHub

`.github/workflows/sign-extension.yml` runs on `v*` tags and also supports a manual retry on a version tag.
It checks that the tag matches the manifest version, runs the checks, and submits through:

```sh
web-ext sign --source-dir extension --channel listed --approval-timeout 0
```

Configure the `mozilla-signing` environment with `AMO_API_KEY` and `AMO_API_SECRET` secrets.
The workflow passes them only to the signing step as `WEB_EXT_API_KEY` and `WEB_EXT_API_SECRET`.
Keep version tags protected and restrict which refs can use that environment.
Require a reviewer if you want manual release approval as well.
Otherwise, a reviewed version tag can trigger submission without another CI confirmation.

Increase the manifest version for every new submission.
Do not blindly retry an upload after a timeout: first check whether AMO already received that version.
Submission can be automatic, but Mozilla's validation and review cannot be bypassed or guaranteed to finish immediately.

## Automatic collection on GitHub after approval

For a listed add-on, `web-ext sign` submits the version but does not return a signed XPI as it does for unlisted signing.
The separate `collect-addon.yml` workflow checks the latest version tag hourly.
You can also run it manually for a particular tag.

Once that exact version is public on AMO, it:

1. Reads the public version metadata from Mozilla.
2. Downloads the immutable file URL and checks Mozilla's SHA256 hash.
3. Checks the XPI's manifest ID and version without repacking it.
4. Publishes the unchanged XPI, `SHA256SUMS`, and `addon.json` as release assets.

`addon.json` contains the URL and SRI file hash for the Nix helper.
Existing assets are compared rather than overwritten.
If approval is still pending, the collector waits for a later scheduled run and publishes nothing.
It requires no AMO credentials, only repository permission to upload release assets.
It collects the latest tag by default. Use the manual tag input for an older release.

The commands also work with other CI systems, through a manual release process:

```sh
# With WEB_EXT_API_KEY and WEB_EXT_API_SECRET supplied by the CI secret store:
nix shell --inputs-from . nixpkgs#web-ext --command \
  web-ext sign --source-dir extension --channel listed --approval-timeout 0

# After AMO publishes the requested version:
python3 tools/fetch_signed_addon.py --version 0.2.0 --output signed-artifacts
```

The collector exits with status 75 while the version is not public.
Its optional `--wait SECONDS` is bounded to 30 minutes.
This polling is release tooling, not part of the installed router.
The workflows were linted but not executed against AMO because no credentials or listing exist yet.

## Install and connect

Regular users install the listed add-on from AMO once it exists.
Firefox then supplies normal add-on updates.
For manual file installation, use **Install Add-on From File** in `about:addons` with Mozilla's signed XPI.

Nix users can pass the same file to `lib.mkSignedExtension` with its fixed hash.
The modules provision it through Firefox policies and disable that extension's independent online updates.
Update its pinned artifact alongside the native package instead.
Other Firefox extensions keep their existing update settings.
See [Nix integration](nix.md).

Install and register the matching native host, then enable the extension in each participating profile.
Its settings must report a connected native bridge.
The nativeMessaging permission is required; no pairing command or key is used.
Set a profile-wide privacy label if needed.
No dedicated Firefox profile is necessary.
Test full browser startup and restored windows with a signed installation before a public release.
Reloading a temporary add-on is not a substitute for that test.

## Preserve the signature

Do not modify or repack Mozilla's returned XPI.
A source rebuild does not reproduce its signature.
The collector's manifest/hash checks are not a replacement for Firefox's cryptographic signature verification.
The Nix helper copies the downloaded file unchanged, and Firefox checks Mozilla's signature on installation.

If you choose unlisted distribution instead, sign through Mozilla with `--channel unlisted` and plan its update mechanism separately.
The automatic release workflows here deliberately use listed distribution for both installation methods.

[amo]: https://addons.mozilla.org/developers/

## Primary references

- [Mozilla signing requirements][signing]
- [web-ext sign command reference][web-ext]
- [Firefox ExtensionSettings policy][policies]

[signing]: https://extensionworkshop.com/documentation/publish/signing-and-distribution-overview/
[web-ext]: https://extensionworkshop.com/documentation/develop/web-ext-command-reference/#web-ext-sign
[policies]: https://mozilla.github.io/policy-templates/#extensionsettings

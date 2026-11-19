# niri Link Router

Open external links in Firefox on the launching window’s [niri][niri] workspace.
If the source workspace is unknown or has no Firefox window, use the configured fallback workspace, `web` by default.
Routed links select the new tab and preserve your current focus. Use `open --bg` to also mark the destination urgent.
If routing is unavailable, the command opens the original URL through Firefox normally.

The Go command talks to a Firefox-owned native host through a private Unix socket.
The extension opens the real page directly. There is no protocol-handler tab or interstitial page.
Existing Firefox profiles work without containers, project maps, or a dedicated routing profile.
Firefox starts one native host per participating profile and closes it when the extension disconnects.
There is no independent service, periodic polling, TCP listener, or routing cache.

## Prerequisites

- Linux with niri and `NIRI_SOCKET` available to applications
- Firefox 142 or later on native Wayland
- A private `XDG_RUNTIME_DIR`, normally `/run/user/UID`
- The native host installed and registered with Firefox
- The router extension enabled in every participating profile

Private browsing windows are not supported by workspace routing.
Sandboxed native-messaging portals, such as Flatpak Firefox, are not supported by the process/profile resolver.
The native host is a separate Go process, owned by Firefox rather than a systemd service.

## Installation

Install [niri Link Router from Firefox Add-ons][amo] in each participating Firefox profile.
Install the companion application with one of the methods below.
The add-on requires the companion application to route links.

### Method 1: Nix Flake (Recommended)

Use Home Manager if it manages your Firefox configuration and MIME defaults.
Import only one router module.
The project repository is [debugloop/niri-link-router on GitHub](https://github.com/debugloop/niri-link-router).
The examples use a local checkout:

```nix
inputs.niri-firefox-router.url = "path:/absolute/path/to/niri-firefox-router";
```

For Home Manager:

```nix
{
  imports = [inputs.niri-firefox-router.homeManagerModules.default];
  programs.niri-firefox-router = {
    enable = true;
    defaultBrowser = true;
    fallbackWorkspace = "web";
  };
}
```

For NixOS:

```nix
{
  imports = [inputs.niri-firefox-router.nixosModules.default];
  programs.niri-firefox-router = {
    enable = true;
    defaultBrowser = true;
    fallbackWorkspace = "web";
  };
}
```

Both modules install the command and native host, register the host with Firefox, and optionally set HTTP/HTTPS defaults.
They do not create Firefox profiles or configure niri bindings.
Home Manager registers both the traditional and XDG user-manifest paths.
The NixOS module uses Firefox's native-host package interface.
Restart Firefox if a changed wrapper needs to expose the new host.

Without `extensionPackage`, install the extension separately.
For optional Nix provisioning, collect the signed artifact as described in
[Firefox installation](docs/firefox-installation.md), then save its `addon.json` as
`firefox-addon.json` beside your configuration:

```nix
{ pkgs, inputs, ... }: let
  addon = builtins.fromJSON (builtins.readFile ./firefox-addon.json);
in {
  programs.niri-firefox-router.extensionPackage =
    inputs.niri-firefox-router.lib.mkSignedExtension {
      inherit pkgs;
      inherit (addon) url hash;
    };
}
```

The helper preserves Mozilla's signed XPI unchanged.
The modules disable independent online updates for this extension only, keeping both halves on the declared version.
See [Nix integration](docs/nix.md) for the module interfaces.

### Method 2: Manual Installation

Build both executables with Go 1.23 or later:

```sh
mkdir -p dist
CGO_ENABLED=0 go build -trimpath -o dist/ ./cmd/niri-firefox-router ./cmd/niri-firefox-router-native-host
python3 tools/install_user.py install \
  --binary dist/niri-firefox-router \
  --native-host dist/niri-firefox-router-native-host \
  --default-browser
```

The installer needs Python 3.
It installs both executables under `~/.local/bin`, registers the native host, and optionally creates niri-specific MIME defaults.
Existing managed `mimeapps.list` links remain unchanged.
The home path must not contain spaces or shell characters because generic `xdg-utils` mishandles quoted executable paths.

To remove the installation and restore previous defaults:

```sh
python3 tools/install_user.py uninstall
```

### Firefox Setup

Install [niri Link Router from Firefox Add-ons][amo] in each participating profile.
Firefox keeps this installation across restarts and supplies add-on updates.

For development only, open `about:debugging#/runtime/this-firefox` and load `extension/manifest.json` as a temporary add-on.
Repeat this in each participating profile.
Firefox removes temporary add-ons on exit, so load the extension again after a restart.

Open the extension settings and check that the native bridge reports **Connected**.
If disconnected, check the native-host installation and select **Reconnect native bridge**.
There are no keys to copy, pairing commands, or approval pages.

## Usage

```sh
niri-firefox-router open 'https://example.org/'
niri-firefox-router open --bg 'https://example.org/'
niri-firefox-router focus
```

For `open`, the router matches the launching process or its ancestors to a Niri window. Callers can pass `--source-window-id ID` before the URL when they know the source. If a launcher detaches, or its process owns windows on different workspaces, the router uses the configured fallback workspace.
`xdg-open` takes the URL and invokes the registered desktop entry. For a background open from a script, call the router directly with `--bg` before the URL. To make every invocation through a desktop entry use background mode, set its command to `niri-firefox-router open --bg %u`.

The `focus` command uses the currently focused workspace first.
With several eligible windows, it prefers the focused or most recently focused window.
Profiles remain separate even when their Firefox window IDs overlap.
A routed success means the extension acknowledged tab creation, not that the network page finished loading.

If no eligible window exists or the bridge is unavailable before submission, `open` invokes Firefox normally with the original URL.
Firefox then chooses the profile and window.
A definite rejection before tab creation also permits ordinary Firefox handling.
If acknowledgement is lost after submission, delivery is uncertain. Do not retry blindly.
The `focus` command never starts Firefox.

### Fallback Workspace

With Nix, set:

```nix
programs.niri-firefox-router.fallbackWorkspace = "browser";
```

Without Nix, create `~/.config/niri-firefox-router/config.json`:

```json
{"fallback_workspace": "browser"}
```

If set, `XDG_CONFIG_HOME` replaces `~/.config`.
A system installation can use `/etc/niri-firefox-router/config.json` with the same format.
The precedence is command-line option, user configuration, system configuration, then `web`.
The command reads configuration on each invocation. A fallback-name change needs no browser restart.

For a single invocation:

```sh
niri-firefox-router open --fallback-workspace browser 'https://example.org/'
```

The same option applies to `focus`.

### Window Labels

Set a profile-wide label in the extension settings, for example `private`.
The title prefix becomes `[nfr:123] private `.
Your niri screenshare rule can match `private`; the numeric ID belongs to the router.
Window Titler must remain disabled because it can overwrite the router prefix.
Labels appear after window creation, so privacy-sensitive rules must fail closed for initially unlabelled windows.

### Kitty-Only Integration

Leave `defaultBrowser = false`, or omit `--default-browser` from the manual installer.
Then set:

```text
open_url_with /absolute/path/to/niri-firefox-router open
```

With Home Manager:

```nix
{ config, lib, ... }: {
  programs.kitty.settings.open_url_with =
    "${lib.getExe config.programs.niri-firefox-router.package} open";
}
```

Reload Kitty's configuration in existing terminals.
This uses the same routing implementation without changing other applications' default browser.

## Troubleshooting

Check window selection without opening a page:

```sh
niri-firefox-router open --dry-run 'https://example.org/'
niri msg --json windows
```

A `"normal_launch": true` result means ordinary Firefox handling.
Dry runs check window selection, not whether a native host is reachable.
For routed delivery, window titles need the `[nfr:<id>]` prefix and extension settings must report a connected bridge.

Check the default handlers:

```sh
xdg-mime query default x-scheme-handler/http
xdg-mime query default x-scheme-handler/https
```

Both report `niri-firefox-router.desktop` when global routing is enabled.
Remove a previous personal installation before relying on Nix defaults.
A desktop entry under `~/.local/share/applications` overrides the package entry even when the handler name is identical.

Receipts in extension settings contain status and time, never URLs.
A disconnected native host gets a bounded reconnect attempt, not a polling loop.
After the attempts end, use **Reconnect native bridge**.

## Upgrade from 0.1

Version 0.2 replaces the old transport completely.
Update the command, native-host registration, and extension together.
The old `pair`, `keygen`, `--key-file`, and `--transport` interfaces no longer exist.
The extension removes its obsolete stored profile key; it never reads the old native master-key file.
After the upgrade works, remove an unused native key file and its persistence entry if you created them for 0.1.
Do not delete the key while an older installation still uses it.
See [the transport decision](docs/adr/0001-native-bridge.md).

## Development Provenance

This project was engineered using AI tools under human direction.

## Development and Releases

```sh
nix develop
nix flake check
nix fmt
```

The extension is plain JavaScript, and both Go executables use only the standard library.
The included GitHub Actions workflows submit version tags to AMO after the owner configures credentials.
A separate workflow collects the published signed artifact and Nix metadata.
Mozilla review is not bypassed by CI. See [Firefox installation](docs/firefox-installation.md).

See [CONTRIBUTING.md](CONTRIBUTING.md), [native validation](docs/native-validation.md), and [security notes](docs/security.md).

[niri]: https://github.com/niri-wm/niri

[amo]: https://addons.mozilla.org/en-US/firefox/addon/niri-link-router/
